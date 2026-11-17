//go:build gui && windows

package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

// guiBuild is true only in claude-profiles-gui.exe (built with -tags gui).
const guiBuild = true

//go:embed ui/index.html
var indexHTML string

const createNoWindow = 0x08000000

type guiProfile struct {
	Label      string `json:"label"`
	Folder     string `json:"folder"`
	CodeFolder string `json:"codeFolder"`
	Created    string `json:"created"`
	LastUsed   string `json:"lastUsed"`
}

type guiState struct {
	Version      string       `json:"version"`
	ProfilesFile string       `json:"profilesFile"`
	LastDir      string       `json:"lastDir"`
	Profiles     []guiProfile `json:"profiles"`
}

type gui struct {
	a       *app
	w       webview2.WebView
	lastDir string
}

func runGUI(recordsFile string) error {
	a, err := openApp(recordsFile)
	if err != nil {
		return err
	}
	scale := enableDPIAwareness()
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(os.Getenv("LOCALAPPDATA"), "Claude-Profiles", "WebView2"),
		WindowOptions: webview2.WindowOptions{
			Title:  "Claude Profiles",
			Width:  uint(780 * scale),
			Height: uint(600 * scale),
			Center: true,
		},
	})
	if w == nil {
		return errors.New("this window needs the Microsoft Edge WebView2 Runtime.\n\nInstall it from https://developer.microsoft.com/microsoft-edge/webview2/ or use claude-profiles.exe in a terminal instead")
	}
	defer w.Destroy()
	w.SetSize(int(560*scale), int(420*scale), webview2.HintMin)

	g := &gui{a: a, w: w, lastDir: os.Getenv("USERPROFILE")}
	// Bound functions run on the UI thread and resolve a JavaScript promise.
	for name, fn := range map[string]interface{}{
		"getState":      g.state,
		"addProfile":    g.add,
		"startDesktop":  g.startDesktop,
		"startCode":     g.startCode,
		"startMain":     g.startMain,
		"removeProfile": g.remove,
		"browseFolder":  g.browseFolder,
		"openFolder":    g.openFolder,
	} {
		if err := w.Bind(name, fn); err != nil {
			return err
		}
	}
	w.SetHtml(indexHTML)
	w.Run()
	return nil
}

func (g *gui) state() guiState {
	s := guiState{Version: version, ProfilesFile: g.a.recordsFile, LastDir: g.lastDir, Profiles: []guiProfile{}}
	for _, r := range g.a.records {
		s.Profiles = append(s.Profiles, guiProfile{r.Label, r.Folder, g.a.codeFolder(r), r.Created, r.LastUsed})
	}
	return s
}

func (g *gui) record(label string) (*Record, error) {
	if r := g.a.find(label); r != nil {
		return r, nil
	}
	return nil, errors.New("that profile no longer exists")
}

func (g *gui) add(label string) (guiState, error) {
	_, err := g.a.createProfile(label)
	return g.state(), err
}

func (g *gui) startDesktop(label string) (guiState, error) {
	r, err := g.record(label)
	if err != nil {
		return g.state(), err
	}
	return g.state(), g.a.launchDesktop(r)
}

func (g *gui) startCode(label, dir string) (guiState, error) {
	r, err := g.record(label)
	if err != nil {
		return g.state(), err
	}
	dir = strings.Trim(strings.TrimSpace(dir), `"`)
	if err := g.a.launchCode(r, dir); err != nil {
		return g.state(), err
	}
	if dir != "" {
		g.lastDir = dir
	}
	return g.state(), nil
}

func (g *gui) startMain() error { return g.a.launchMain() }

func (g *gui) remove(label string) (guiState, error) {
	r, err := g.record(label)
	if err != nil {
		return g.state(), err
	}
	return g.state(), g.a.deleteProfile(r)
}

func (g *gui) openFolder(path string) error {
	if !exists(path) {
		return errors.New("that folder does not exist yet; it is created the first time the profile is started")
	}
	return exec.Command("explorer.exe", path).Start()
}

// browseFolder shows the Windows folder picker without blocking the UI thread,
// then calls window.onFolderPicked(path) in the page ("" when cancelled).
func (g *gui) browseFolder(start string) {
	if start == "" {
		start = g.lastDir
	}
	go func() {
		script := strings.Join([]string{
			"Add-Type -AssemblyName System.Windows.Forms",
			"[Console]::OutputEncoding = [Text.Encoding]::UTF8",
			"$owner = New-Object System.Windows.Forms.Form -Property @{ TopMost = $true; ShowInTaskbar = $false }",
			"$d = New-Object System.Windows.Forms.FolderBrowserDialog",
			"$d.Description = 'Choose the project folder for Claude Code'",
			"$d.ShowNewFolderButton = $true",
			"$d.SelectedPath = " + psQuote(start),
			"if ($d.ShowDialog($owner) -eq 'OK') { [Console]::Out.Write($d.SelectedPath) }",
		}, "; ")
		cmd := exec.Command("powershell.exe", "-NoProfile", "-STA", "-EncodedCommand", encodePS(script))
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
		out, _ := cmd.Output()
		picked, _ := json.Marshal(strings.TrimSpace(string(out)))
		g.w.Dispatch(func() { g.w.Eval("window.onFolderPicked && window.onFolderPicked(" + string(picked) + ")") })
	}()
}

var (
	user32                            = syscall.NewLazyDLL("user32.dll")
	procMessageBox                    = user32.NewProc("MessageBoxW")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
)

// enableDPIAwareness makes Windows render the window at the display's real
// resolution instead of stretching a 96-DPI bitmap (blurry text on scaled
// displays). It returns the display scale (1.0 at 100%, 2.0 at 200%), since
// window sizes are then in physical pixels.
func enableDPIAwareness() float64 {
	const perMonitorAwareV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	if procSetProcessDpiAwarenessContext.Find() == nil {
		procSetProcessDpiAwarenessContext.Call(perMonitorAwareV2) // Windows 10 1703+
	} else if procSetProcessDPIAware.Find() == nil {
		procSetProcessDPIAware.Call()
	}
	if procGetDpiForSystem.Find() == nil {
		if dpi, _, _ := procGetDpiForSystem.Call(); dpi >= 96 {
			return float64(dpi) / 96
		}
	}
	return 1
}

func showError(msg string) {
	text, _ := syscall.UTF16PtrFromString(msg)
	title, _ := syscall.UTF16PtrFromString("Claude Profiles")
	const mbIconError = 0x10
	procMessageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), mbIconError)
}
