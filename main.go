// Command claude-profiles runs Claude Desktop and Claude Code with separate
// accounts side by side on Windows, macOS and Linux. It shows every Claude
// profile (user) you have, lets you add a new one by email, and starts the one
// you pick with its own login, settings and data:
//
//   - Claude Desktop in its own window (--user-data-dir); Windows and macOS
//   - Claude Code with its own config folder (CLAUDE_CONFIG_DIR); all three.
//     On macOS and Linux it runs in the current terminal when started from one.
//
// Usage:
//
//	claude-profiles                                  open the menu
//	claude-profiles -profile a@second.com            start that user's Claude Desktop
//	claude-profiles -profile a@second.com -code      start that user's Claude Code in the current folder
//	claude-profiles -profile a@second.com -code -dir ~/src/app
//	claude-profiles -profile a@second.com -shortcut both    Desktop + menu shortcuts (add -code for Claude Code)
//	claude-profiles -profile a@second.com -rename b@second.com   rename (data and sign-in kept)
//	claude-profiles -list                            list the profiles and exit
package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf16"
)

// version is set at build time by GoReleaser (-X main.version=...).
var version = "dev"

// Record is one profile, stored in profiles.json. (Files written by the old
// PowerShell version of this tool use the same shape and still load.)
type Record struct {
	Label    string `json:"label"`
	Folder   string `json:"folder"`
	Created  string `json:"created"`
	LastUsed string `json:"lastUsed"`
	// RecentDirs are the latest Claude Code project folders, newest first.
	RecentDirs []string `json:"recentDirs,omitempty"`
}

type app struct {
	dataRoot     string
	codeDataRoot string
	recordsFile  string
	records      []*Record
	in           *bufio.Reader
	eof          bool // stdin is closed
	codeHere     bool // run Claude Code in this terminal (macOS/Linux CLI)
}

// Where shortcuts can be placed: the Desktop, or the system's app launcher
// (Windows Start menu, macOS Applications folder, Linux app menu).
const (
	placeDesktop = "desktop"
	placeStart   = "start"
)

var (
	mainStoreFolder = regexp.MustCompile(`^claude_[a-z0-9]+$`)
	unsafeChars     = regexp.MustCompile(`[^A-Za-z0-9._-]`)
	codeChoice      = regexp.MustCompile(`^[Cc]\s*(\d+)$`)
	numberChoice    = regexp.MustCompile(`^\d+$`)
)

func main() {
	profile := flag.String("profile", "", "start this user directly, no menu")
	code := flag.Bool("code", false, "with -profile: start Claude Code instead of Claude Desktop")
	dir := flag.String("dir", "", "with -code: project folder to open (default: current folder)")
	list := flag.Bool("list", false, "list the profiles and exit")
	records := flag.String("profiles-file", "", "path to profiles.json (default: next to the .exe on Windows, ~/.config/claude-profiles on Linux, ~/Library/Application Support/Claude Profiles on macOS)")
	shortcut := flag.String("shortcut", "", "with -profile: create a shortcut instead of starting it: desktop, start or both")
	rename := flag.String("rename", "", "with -profile: give that user this new email or name (data and sign-in are kept)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("claude-profiles", version)
		return
	}

	interactive := *profile == "" && !*list
	if guiBuild && interactive && *shortcut == "" && *rename == "" {
		// claude-profiles-gui.exe without arguments opens the window.
		if err := runGUI(*records); err != nil {
			showError(err.Error())
			os.Exit(1)
		}
		return
	}
	opts := options{profile: *profile, code: *code, dir: *dir, list: *list, recordsFile: *records, shortcut: *shortcut, rename: *rename}
	if err := run(opts); err != nil {
		if guiBuild { // no console to print to
			showError(err.Error())
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		if interactive {
			// Keep a double-clicked window open long enough to read the error.
			fmt.Print("Press Enter to exit")
			bufio.NewReader(os.Stdin).ReadString('\n')
		}
		os.Exit(1)
	}
}

// openApp loads profiles.json and adds any profile folders found on disk.
func openApp(recordsFile string) (*app, error) {
	a, err := newApp(recordsFile)
	if err != nil {
		return nil, err
	}
	if err := a.load(); err != nil {
		return nil, err
	}
	a.discover()
	if err := a.save(); err != nil {
		return nil, err
	}
	return a, nil
}

// options are the command-line flags.
type options struct {
	profile, dir, recordsFile, shortcut, rename string
	code, list                                  bool
}

func run(o options) error {
	if o.shortcut != "" && o.profile == "" {
		return errors.New("-shortcut needs -profile")
	}
	if o.rename != "" && o.profile == "" {
		return errors.New("-rename needs -profile")
	}
	a, err := openApp(o.recordsFile)
	if err != nil {
		return err
	}
	a.codeHere = codeRunsInThisTerminal()

	switch {
	case o.list:
		a.printList()
		return nil
	case o.profile != "":
		r := a.find(o.profile)
		if r == nil {
			return fmt.Errorf("no user named %q; run without arguments to see the list or add it", o.profile)
		}
		if o.rename != "" {
			old := r.Label
			if err := a.renameProfile(r, o.rename); err != nil {
				return err
			}
			fmt.Printf("Renamed %s to %s\n", old, r.Label)
			return nil
		}
		if o.shortcut != "" {
			return a.makeShortcuts(r, o.shortcut, o.code, o.dir)
		}
		if o.code {
			dir := o.dir
			if dir == "" {
				if dir, err = os.Getwd(); err != nil {
					return err
				}
			}
			return a.launchCode(r, dir)
		}
		return a.launchDesktop(r)
	default:
		return a.menu()
	}
}

// makeShortcuts handles -shortcut desktop|start|both.
func (a *app) makeShortcuts(r *Record, where string, code bool, dir string) error {
	var places []string
	switch strings.ToLower(where) {
	case placeDesktop:
		places = []string{placeDesktop}
	case placeStart, "startmenu", "start-menu":
		places = []string{placeStart}
	case "both":
		places = []string{placeDesktop, placeStart}
	default:
		return fmt.Errorf("-shortcut must be desktop, start or both, not %q", where)
	}
	for _, p := range places {
		path, err := a.createShortcut(r, p, code, dir)
		if err != nil {
			return err
		}
		fmt.Println("Created", path)
	}
	return nil
}

func newApp(recordsFile string) (*app, error) {
	desktopRoot, codeRoot, err := dataRoots()
	if err != nil {
		return nil, err
	}
	if recordsFile == "" {
		if recordsFile, err = defaultRecordsFile(); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(recordsFile), 0o755); err != nil {
		return nil, err
	}
	return &app{
		dataRoot:     desktopRoot,
		codeDataRoot: codeRoot,
		recordsFile:  recordsFile,
		in:           bufio.NewReader(os.Stdin),
	}, nil
}

// ---------------------------------------------------------------- records

func (a *app) load() error {
	raw, err := os.ReadFile(a.recordsFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// Windows PowerShell 5.1 writes UTF-8 with a BOM.
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	var recs []*Record
	if raw[0] == '{' { // a single object, as older PowerShell output can produce
		var one Record
		if err := json.Unmarshal(raw, &one); err != nil {
			return fmt.Errorf("reading %s: %w", a.recordsFile, err)
		}
		recs = []*Record{&one}
	} else if err := json.Unmarshal(raw, &recs); err != nil {
		return fmt.Errorf("reading %s: %w", a.recordsFile, err)
	}
	for _, r := range recs {
		if r != nil && r.Label != "" {
			a.records = append(a.records, r)
		}
	}
	return nil
}

func (a *app) save() error {
	recs := a.records
	if recs == nil {
		recs = []*Record{}
	}
	out, err := json.MarshalIndent(recs, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.recordsFile, append(out, '\r', '\n'), 0o644)
}

// discover adds any folder in Claude-Data that is not the main Store profile and
// not yet recorded, so nothing already on disk is hidden from the menu.
func (a *app) discover() {
	entries, err := os.ReadDir(a.dataRoot)
	if err != nil {
		return
	}
	known := map[string]bool{}
	for _, r := range a.records {
		known[pathKey(r.Folder)] = true
	}
	for _, e := range entries {
		if !e.IsDir() || mainStoreFolder.MatchString(e.Name()) {
			continue
		}
		full := filepath.Join(a.dataRoot, e.Name())
		if known[pathKey(full)] {
			continue
		}
		created := ""
		if info, err := e.Info(); err == nil {
			created = info.ModTime().Format("2006-01-02")
		}
		a.records = append(a.records, &Record{Label: e.Name(), Folder: full, Created: created})
	}
}

func (a *app) find(label string) *Record {
	for _, r := range a.records {
		if strings.EqualFold(r.Label, label) {
			return r
		}
	}
	return nil
}

func (a *app) codeFolder(r *Record) string {
	// Claude Code config lives in a parallel root with the same folder name.
	return filepath.Join(a.codeDataRoot, filepath.Base(r.Folder))
}

func (a *app) printList() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "LABEL\tDESKTOP\tFOLDER\tCODE FOLDER\tCREATED\tLAST USED")
	for _, r := range a.records {
		state := "-"
		if desktopRunning(r.Folder) {
			state = "running"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Label, state, r.Folder, a.codeFolder(r), r.Created, r.LastUsed)
	}
	w.Flush()
}

func (a *app) touch(r *Record) error {
	r.LastUsed = time.Now().Format("2006-01-02 15:04")
	return a.save()
}

// ---------------------------------------------------------------- launching

func (a *app) launchDesktop(r *Record) error {
	cmd, err := desktopCommand(r.Folder)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(r.Folder, 0o755); err != nil {
		return err
	}
	if err := startDetached(cmd, false); err != nil {
		return err
	}
	fmt.Printf("Started Claude Desktop as %s\n", r.Label)
	return a.touch(r)
}

func (a *app) launchMain() error {
	cmd, err := desktopCommand("")
	if err != nil {
		return err
	}
	if err := startDetached(cmd, false); err != nil {
		return err
	}
	fmt.Println("Started your main Claude Desktop")
	return nil
}

// launchCode starts Claude Code as r in workDir (blank: the home folder). From
// a terminal on macOS and Linux it runs right there; otherwise it opens a new
// terminal window.
func (a *app) launchCode(r *Record, workDir string) error {
	claude, err := findClaudeCode()
	if err != nil {
		return err
	}
	configDir := a.codeFolder(r)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	if workDir == "" {
		if workDir, err = os.UserHomeDir(); err != nil {
			return err
		}
	}
	if workDir, err = filepath.Abs(workDir); err != nil {
		return err
	}
	if info, err := os.Stat(workDir); err != nil || !info.IsDir() {
		return fmt.Errorf("folder not found: %s", workDir)
	}

	if a.codeHere {
		r.addRecentDir(workDir)
		if err := a.touch(r); err != nil {
			return err
		}
		fmt.Printf("Claude Code as %s in %s\n", r.Label, workDir)
		return runCodeHere(claude, configDir, workDir) // only returns on failure
	}
	if err := openCodeTerminal("Claude Code - "+r.Label, claude, configDir, workDir); err != nil {
		return err
	}
	fmt.Printf("Started Claude Code as %s in %s\n", r.Label, workDir)
	r.addRecentDir(workDir)
	return a.touch(r)
}

// maxRecentDirs is how many Claude Code project folders each profile remembers.
const maxRecentDirs = 5

// addRecentDir moves dir to the front of the profile's recent Claude Code
// folders (ignoring trailing slashes, and letter case on Windows and macOS),
// keeping at most maxRecentDirs.
func (r *Record) addRecentDir(dir string) {
	dir = filepath.Clean(dir)
	list := []string{dir}
	for _, d := range r.RecentDirs {
		if pathKey(d) != pathKey(dir) && len(list) < maxRecentDirs {
			list = append(list, d)
		}
	}
	r.RecentDirs = list
}

// forgetRecentDir removes dir from the profile's recent folders.
func (r *Record) forgetRecentDir(dir string) {
	var list []string
	for _, d := range r.RecentDirs {
		if pathKey(d) != pathKey(dir) {
			list = append(list, d)
		}
	}
	r.RecentDirs = list
}

// existingRecentDirs returns the recent folders that still exist, so moved or
// deleted projects aren't offered.
func (r *Record) existingRecentDirs() []string {
	list := []string{}
	for _, d := range r.RecentDirs {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			list = append(list, d)
		}
	}
	return list
}

func startDetached(cmd *exec.Cmd, newConsole bool) error {
	setNewConsole(cmd, newConsole)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// psQuote makes a PowerShell single-quoted string literal.
func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// encodePS encodes a script for powershell.exe -EncodedCommand (base64 of UTF-16LE).
func encodePS(s string) string {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		b[2*i] = byte(v)
		b[2*i+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// ---------------------------------------------------------------- menu

func (a *app) prompt(msg string) string {
	fmt.Print(msg)
	line, err := a.in.ReadString('\n')
	if err != nil && line == "" {
		a.eof = true
	}
	// Input piped from Windows PowerShell can start with a byte order mark.
	return strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
}

func (a *app) menu() error {
	for {
		a.showMenu()
		choice := a.prompt("Type a number or letter, then Enter: ")
		switch {
		case a.eof:
			fmt.Println()
			return nil // input closed: nothing more to read
		case strings.EqualFold(choice, "q"):
			return nil
		case strings.EqualFold(choice, "m"):
			return a.launchMain()
		case strings.EqualFold(choice, "a"):
			launched, err := a.addProfile()
			if err != nil || launched {
				return err
			}
		case strings.EqualFold(choice, "r"):
			if err := a.removeProfile(); err != nil {
				return err
			}
			a.prompt("Press Enter to continue")
		case strings.EqualFold(choice, "n"):
			if err := a.renamePrompt(); err != nil {
				fmt.Println("Error:", err)
			}
			a.prompt("Press Enter to continue")
		case strings.EqualFold(choice, "s"):
			if err := a.shortcutPrompt(); err != nil {
				fmt.Println("Error:", err)
			}
			a.prompt("Press Enter to continue")
		case codeChoice.MatchString(choice):
			n, _ := strconv.Atoi(codeChoice.FindStringSubmatch(choice)[1])
			launched, err := a.pickCode(n - 1)
			if err != nil || launched {
				return err
			}
		case numberChoice.MatchString(choice):
			n, _ := strconv.Atoi(choice)
			if n >= 1 && n <= len(a.records) {
				return a.launchDesktop(a.records[n-1])
			}
			fmt.Println("No such number.")
			time.Sleep(time.Second)
		}
	}
}

func (a *app) showMenu() {
	clearScreen()
	fmt.Println()
	fmt.Println("  Claude - choose a user")
	fmt.Println("  ----------------------")
	if len(a.records) == 0 {
		fmt.Println("  (no extra users yet - press A to add one)")
	}
	for i, r := range a.records {
		when := "never used"
		if r.LastUsed != "" {
			when = "last used " + r.LastUsed
		}
		if desktopRunning(r.Folder) {
			when = "* running   " + when
		}
		fmt.Printf("  %2d)  %-32s %s\n", i+1, r.Label, when)
	}
	fmt.Println()
	desktop := desktopAvailable()
	if desktop {
		fmt.Println("   1, 2, ...   Claude Desktop as that user (* = already open)")
	}
	where := "opens a terminal"
	if a.codeHere {
		where = "runs in this terminal"
	}
	fmt.Printf("  C1, C2, ...  Claude Code as that user (%s)\n", where)
	fmt.Println()
	if desktop {
		fmt.Println("   M)  Main Claude Desktop (your normal profile)")
	}
	fmt.Println("   A)  Add a new user")
	fmt.Println("   N)  Rename a user")
	fmt.Println("   R)  Remove a user")
	fmt.Printf("   S)  Create a Desktop / %s shortcut\n", placeLabel(placeStart))
	fmt.Println("   Q)  Quit")
	fmt.Println()
}

func (a *app) pickCode(i int) (bool, error) {
	if i < 0 || i >= len(a.records) {
		fmt.Println("No such number.")
		time.Sleep(time.Second)
		return false, nil
	}
	dir := a.promptDir(a.records[i])
	return true, a.launchCode(a.records[i], dir)
}

// promptDir asks for a Claude Code project folder, offering the profile's
// recent folders by number.
func (a *app) promptDir(r *Record) string {
	recent := r.existingRecentDirs()
	if len(recent) == 0 {
		return strings.Trim(a.prompt("Project folder for Claude Code (blank = your home folder): "), `"`)
	}
	fmt.Println("Recent folders:")
	for i, d := range recent {
		fmt.Printf("  %d)  %s\n", i+1, d)
	}
	ans := strings.Trim(a.prompt("Type a number, another folder, or leave blank for your home folder: "), `"`)
	if n, err := strconv.Atoi(ans); err == nil && n >= 1 && n <= len(recent) {
		return recent[n-1]
	}
	return ans
}

func safeFolderName(label string) string {
	s := strings.Trim(unsafeChars.ReplaceAllString(strings.TrimSpace(label), "_"), "._-")
	if s == "" {
		return "profile"
	}
	return s
}

var errExists = errors.New("a user with that name already exists")

// createProfile records a new user and creates its Claude Desktop data folder.
func (a *app) createProfile(label string) (*Record, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil, errors.New("enter an email or name")
	}
	if a.find(label) != nil {
		return nil, errExists
	}
	safe := safeFolderName(label)
	folder := filepath.Join(a.dataRoot, safe)
	for n := 2; exists(folder); n++ {
		folder = filepath.Join(a.dataRoot, fmt.Sprintf("%s-%d", safe, n))
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return nil, err
	}
	r := &Record{Label: label, Folder: folder, Created: time.Now().Format("2006-01-02")}
	a.records = append(a.records, r)
	return r, a.save()
}

// renameProfile changes a user's label. Its data folders stay where they are
// (so it stays signed in); its shortcuts are renamed to match. The record is
// saved first, so a shortcut problem is reported without undoing the rename.
func (a *app) renameProfile(r *Record, newLabel string) error {
	newLabel = strings.TrimSpace(newLabel)
	if newLabel == "" {
		return errors.New("enter an email or name")
	}
	if newLabel == r.Label {
		return nil
	}
	if other := a.find(newLabel); other != nil && other != r {
		return errExists
	}
	oldLabel := r.Label
	r.Label = newLabel
	if err := a.save(); err != nil {
		r.Label = oldLabel
		return err
	}
	if err := renameShortcuts(oldLabel, newLabel); err != nil {
		return fmt.Errorf("renamed, but %w", err)
	}
	return nil
}

// deleteProfile removes a user's Desktop and Code data folders, its shortcuts
// and its record.
func (a *app) deleteProfile(r *Record) error {
	removeShortcuts(r.Label)
	for _, d := range []string{r.Folder, a.codeFolder(r)} {
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}
	for i, x := range a.records {
		if x == r {
			a.records = append(a.records[:i], a.records[i+1:]...)
			break
		}
	}
	return a.save()
}

func (a *app) addProfile() (bool, error) {
	label := a.prompt("Email or name for the new user (blank to cancel): ")
	if label == "" {
		return false, nil
	}
	r, err := a.createProfile(label)
	if errors.Is(err, errExists) {
		fmt.Printf("A user named %s already exists.\n", label)
		time.Sleep(2 * time.Second)
		return false, nil
	}
	if err != nil {
		return false, err
	}
	fmt.Printf("Added %s  ->  %s\n", label, r.Folder)

	switch ans := strings.ToLower(a.prompt("Start it now and sign in? [D]esktop / [C]ode / [N]o (default D): ")); {
	case ans == "" || strings.HasPrefix(ans, "d") || strings.HasPrefix(ans, "y"):
		return true, a.launchDesktop(r)
	case strings.HasPrefix(ans, "c"):
		return a.pickCode(len(a.records) - 1)
	}
	return false, nil
}

func (a *app) renamePrompt() error {
	n, err := strconv.Atoi(a.prompt("Number of the user to rename (blank to cancel): "))
	if err != nil {
		return nil
	}
	if n < 1 || n > len(a.records) {
		fmt.Println("No such number.")
		return nil
	}
	r := a.records[n-1]
	newLabel := a.prompt(fmt.Sprintf("New email or name for %s (blank to cancel): ", r.Label))
	if newLabel == "" {
		return nil
	}
	old := r.Label
	if err := a.renameProfile(r, newLabel); err != nil {
		return err
	}
	fmt.Printf("Renamed %s to %s. It stays signed in.\n", old, r.Label)
	return nil
}

func (a *app) shortcutPrompt() error {
	n, err := strconv.Atoi(a.prompt("Number of the user (blank to cancel): "))
	if err != nil {
		return nil
	}
	if n < 1 || n > len(a.records) {
		fmt.Println("No such number.")
		return nil
	}
	r := a.records[n-1]
	code := strings.HasPrefix(strings.ToLower(a.prompt("Shortcut for [D]esktop app or Claude [C]ode? (default D): ")), "c")
	dir := ""
	if code {
		dir = a.promptDir(r)
	}
	where := "both"
	switch strings.ToLower(a.prompt("Put it on the [D]esktop, in the [S]tart menu, or [B]oth? (default B): ")) {
	case "d":
		where = placeDesktop
	case "s":
		where = placeStart
	}
	return a.makeShortcuts(r, where, code, dir)
}

func (a *app) removeProfile() error {
	pick := a.prompt("Number of the user to remove (blank to cancel): ")
	n, err := strconv.Atoi(pick)
	if err != nil {
		return nil
	}
	if n < 1 || n > len(a.records) {
		fmt.Println("No such number.")
		return nil
	}
	r := a.records[n-1]
	codeDir := a.codeFolder(r)
	fmt.Printf("This removes %s and deletes its data folders:\n", r.Label)
	fmt.Printf("  %s   (Claude Desktop)\n", r.Folder)
	fmt.Printf("  %s   (Claude Code)\n", codeDir)
	fmt.Printf("Its Desktop and %s shortcuts are removed too.\n", placeLabel(placeStart))
	fmt.Println("That signs the user out on this PC and cannot be undone. Conversations stay on the Claude account.")
	if a.prompt("Type DELETE to confirm, anything else to cancel: ") != "DELETE" {
		fmt.Println("Cancelled.")
		return nil
	}
	if err := a.deleteProfile(r); err != nil {
		return err
	}
	fmt.Printf("Removed %s.\n", r.Label)
	return nil
}

// ---------------------------------------------------------------- helpers

func exists(p string) bool {
	_, err := os.Lstat(p) // Lstat: app execution aliases are reparse points
	return err == nil
}
