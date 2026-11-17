//go:build gui && windows

package main

import (
	_ "embed"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

//go:embed winres/icon.ico
var iconICO []byte

const createNoWindow = 0x08000000

// newWindow opens the WebView2 window, sized for the display's scaling.
func newWindow() (view, error) {
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
		return nil, errors.New("this window needs the Microsoft Edge WebView2 Runtime.\n\nInstall it from https://developer.microsoft.com/microsoft-edge/webview2/ or use claude-profiles.exe in a terminal instead")
	}
	w.SetSize(int(560*scale), int(420*scale), webview2.HintMin)
	setWindowIcon(uintptr(w.Window()))
	return w, nil
}

// pickFolder shows the Windows folder picker in a separate process, so the
// window keeps responding, and calls done on the UI thread.
func pickFolder(w view, start string, done func(string)) {
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
		picked := strings.TrimSpace(string(out))
		w.Dispatch(func() { done(picked) })
	}()
}

func openInFileManager(path string) error { return exec.Command("explorer.exe", path).Start() }

var (
	user32                            = syscall.NewLazyDLL("user32.dll")
	procMessageBox                    = user32.NewProc("MessageBoxW")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procCreateIconFromResourceEx      = user32.NewProc("CreateIconFromResourceEx")
	procSendMessage                   = user32.NewProc("SendMessageW")
)

// setWindowIcon sets the title bar and taskbar icon from the embedded .ico,
// picking the entry closest to each size Windows asks for.
func setWindowIcon(hwnd uintptr) {
	const (
		wmSetIcon  = 0x0080
		iconSmall  = 0
		iconBig    = 1
		smCxIcon   = 11
		smCxSmIcon = 49
	)
	for _, which := range []struct{ kind, metric uintptr }{{iconBig, smCxIcon}, {iconSmall, smCxSmIcon}} {
		want, _, _ := procGetSystemMetrics.Call(which.metric)
		bits := icoEntry(iconICO, int(want))
		if bits == nil {
			continue
		}
		h, _, _ := procCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&bits[0])), uintptr(len(bits)), 1, 0x00030000, want, want, 0)
		if h != 0 {
			procSendMessage.Call(hwnd, wmSetIcon, which.kind, h)
		}
	}
}

// icoEntry returns the image data of the smallest icon at least `want` pixels
// wide (or the largest one), from a .ico file.
func icoEntry(ico []byte, want int) []byte {
	if len(ico) < 6 {
		return nil
	}
	count := int(ico[4]) | int(ico[5])<<8
	best, bestSize := -1, 0
	for i := 0; i < count && 6+16*(i+1) <= len(ico); i++ {
		size := int(ico[6+16*i])
		if size == 0 {
			size = 256
		}
		better := best < 0 ||
			(size >= want && (bestSize < want || size < bestSize)) ||
			(bestSize < want && size > bestSize)
		if better {
			best, bestSize = i, size
		}
	}
	if best < 0 {
		return nil
	}
	e := ico[6+16*best:]
	n := int(e[8]) | int(e[9])<<8 | int(e[10])<<16 | int(e[11])<<24
	off := int(e[12]) | int(e[13])<<8 | int(e[14])<<16 | int(e[15])<<24
	if off < 0 || n <= 0 || off+n > len(ico) {
		return nil
	}
	return ico[off : off+n]
}

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
