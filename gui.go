//go:build gui && (windows || linux)

package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
)

// guiBuild is true only in claude-profiles-gui (built with -tags gui).
const guiBuild = true

//go:embed ui/index.html
var indexHTML string

// view is the part of the webview API that the Windows (WebView2) and Linux
// (WebKitGTK) libraries share. Bound functions run on the UI thread and
// resolve a JavaScript promise.
type view interface {
	Bind(name string, f interface{}) error
	Dispatch(f func())
	Eval(js string)
	SetHtml(html string)
	Run()
	Destroy()
}

type guiProfile struct {
	Label      string   `json:"label"`
	Folder     string   `json:"folder"`
	CodeFolder string   `json:"codeFolder"`
	Created    string   `json:"created"`
	LastUsed   string   `json:"lastUsed"`
	Running    bool     `json:"running"`    // Claude Desktop is open for this profile
	RecentDirs []string `json:"recentDirs"` // recent Claude Code folders that still exist
}

type guiState struct {
	Version          string       `json:"version"`
	Platform         string       `json:"platform"`
	DesktopAvailable bool         `json:"desktopAvailable"` // false on Linux without an unofficial build
	StartLabel       string       `json:"startLabel"`       // "Start menu", "app menu", …
	ProfilesFile     string       `json:"profilesFile"`
	LastDir          string       `json:"lastDir"`
	Profiles         []guiProfile `json:"profiles"`
}

type gui struct {
	a       *app
	w       view
	lastDir string
}

func runGUI(recordsFile string) error {
	a, err := openApp(recordsFile)
	if err != nil {
		return err
	}
	w, err := newWindow()
	if err != nil {
		return err
	}
	defer w.Destroy()

	home, _ := os.UserHomeDir()
	g := &gui{a: a, w: w, lastDir: home}
	for name, fn := range map[string]interface{}{
		"getState":      g.state,
		"addProfile":    g.add,
		"startDesktop":  g.startDesktop,
		"startCode":     g.startCode,
		"startMain":     g.startMain,
		"removeProfile": g.remove,
		"makeShortcuts": g.makeShortcuts,
		"renameProfile": g.rename,
		"forgetRecent":  g.forgetRecent,
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
	s := guiState{
		Version:          version,
		Platform:         runtime.GOOS,
		DesktopAvailable: desktopAvailable(),
		StartLabel:       placeLabel(placeStart),
		ProfilesFile:     g.a.recordsFile,
		LastDir:          g.lastDir,
		Profiles:         []guiProfile{},
	}
	for _, r := range g.a.records {
		s.Profiles = append(s.Profiles, guiProfile{r.Label, r.Folder, g.a.codeFolder(r), r.Created, r.LastUsed, desktopRunning(r.Folder), r.existingRecentDirs()})
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

func (g *gui) rename(label, newLabel string) (guiState, error) {
	r, err := g.record(label)
	if err != nil {
		return g.state(), err
	}
	return g.state(), g.a.renameProfile(r, newLabel)
}

// forgetRecent drops one folder from a profile's recent Claude Code folders.
func (g *gui) forgetRecent(label, dir string) (guiState, error) {
	r, err := g.record(label)
	if err != nil {
		return g.state(), err
	}
	r.forgetRecentDir(dir)
	return g.state(), g.a.save()
}

func (g *gui) remove(label string) (guiState, error) {
	r, err := g.record(label)
	if err != nil {
		return g.state(), err
	}
	return g.state(), g.a.deleteProfile(r)
}

// makeShortcuts creates the chosen shortcuts and returns where they went
// ("desktop", "start").
func (g *gui) makeShortcuts(label string, code bool, dir string, desktop, start bool) ([]string, error) {
	r, err := g.record(label)
	if err != nil {
		return nil, err
	}
	var places []string
	if desktop {
		places = append(places, placeDesktop)
	}
	if start {
		places = append(places, placeStart)
	}
	if len(places) == 0 {
		return nil, errors.New("choose Desktop, " + placeLabel(placeStart) + ", or both")
	}
	var made []string
	for _, p := range places {
		if _, err := g.a.createShortcut(r, p, code, dir); err != nil {
			return made, err
		}
		made = append(made, p)
	}
	if code && strings.TrimSpace(dir) != "" {
		g.lastDir = strings.Trim(strings.TrimSpace(dir), `"`)
	}
	return made, nil
}

func (g *gui) openFolder(path string) error {
	if !exists(path) {
		return errors.New("that folder does not exist yet; it is created the first time the profile is started")
	}
	return openInFileManager(path)
}

// browseFolder shows the system folder picker, then calls
// window.onFolderPicked(path) in the page ("" when cancelled).
func (g *gui) browseFolder(start string) {
	if start == "" {
		start = g.lastDir
	}
	pickFolder(g.w, start, func(picked string) {
		js, _ := json.Marshal(picked)
		g.w.Eval("window.onFolderPicked && window.onFolderPicked(" + string(js) + ")")
	})
}
