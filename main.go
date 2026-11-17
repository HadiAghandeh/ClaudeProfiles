// Command claude-profiles is the Go version of the Claude Desktop and Claude Code
// profile launcher. It shows every Claude profile (user) you have, lets you add a
// new one by email, and starts the one you pick with its own login, settings and
// data:
//
//   - Claude Desktop in its own window (--user-data-dir)
//   - Claude Code in its own terminal (CLAUDE_CONFIG_DIR)
//
// It reads and writes the same profiles.json and data folders as
// claude-profiles.ps1, so both versions can be used side by side.
//
// Usage:
//
//	claude-profiles                                  open the menu
//	claude-profiles -profile a@second.com            start that user's Claude Desktop
//	claude-profiles -profile a@second.com -code      start that user's Claude Code in the current folder
//	claude-profiles -profile a@second.com -code -dir C:\src\app
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

// Record is one profile, stored in profiles.json in the same shape the
// PowerShell script uses.
type Record struct {
	Label    string `json:"label"`
	Folder   string `json:"folder"`
	Created  string `json:"created"`
	LastUsed string `json:"lastUsed"`
}

type app struct {
	dataRoot     string
	codeDataRoot string
	recordsFile  string
	records      []*Record
	in           *bufio.Reader
	eof          bool // stdin is closed
}

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
	records := flag.String("profiles-file", "", "path to profiles.json (default: next to the executable)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("claude-profiles", version)
		return
	}

	interactive := *profile == "" && !*list
	if err := run(*profile, *code, *dir, *list, *records); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		if interactive {
			// Keep a double-clicked window open long enough to read the error.
			fmt.Print("Press Enter to exit")
			bufio.NewReader(os.Stdin).ReadString('\n')
		}
		os.Exit(1)
	}
}

func run(profile string, code bool, dir string, list bool, recordsFile string) error {
	a, err := newApp(recordsFile)
	if err != nil {
		return err
	}
	if err := a.load(); err != nil {
		return err
	}
	a.discover()
	if err := a.save(); err != nil {
		return err
	}

	switch {
	case list:
		a.printList()
		return nil
	case profile != "":
		r := a.find(profile)
		if r == nil {
			return fmt.Errorf("no user named %q; run without arguments to see the list or add it", profile)
		}
		if code {
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

func newApp(recordsFile string) (*app, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil, errors.New("LOCALAPPDATA is not set; this tool runs on Windows")
	}
	if recordsFile == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		recordsFile = filepath.Join(filepath.Dir(exe), "profiles.json")
	}
	return &app{
		dataRoot:     filepath.Join(local, "Claude-Data"),
		codeDataRoot: filepath.Join(local, "Claude-Code-Data"),
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
		known[strings.ToLower(r.Folder)] = true
	}
	for _, e := range entries {
		if !e.IsDir() || mainStoreFolder.MatchString(e.Name()) {
			continue
		}
		full := filepath.Join(a.dataRoot, e.Name())
		if known[strings.ToLower(full)] {
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
	fmt.Fprintln(w, "LABEL\tFOLDER\tCODE FOLDER\tCREATED\tLAST USED")
	for _, r := range a.records {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Label, r.Folder, a.codeFolder(r), r.Created, r.LastUsed)
	}
	w.Flush()
}

func (a *app) touch(r *Record) error {
	r.LastUsed = time.Now().Format("2006-01-02 15:04")
	return a.save()
}

// ---------------------------------------------------------------- launching

func findClaudeDesktop() (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	for _, c := range []string{
		filepath.Join(local, `Microsoft\WindowsApps\claude-desktop.exe`),
		filepath.Join(local, `Programs\Claude\Claude.exe`),
		filepath.Join(local, `AnthropicClaude\Claude.exe`),
	} {
		if exists(c) {
			return c, nil
		}
	}
	return "", errors.New("Claude Desktop was not found; install it from https://claude.ai/download")
}

func findClaudeCode() (string, error) {
	if p, err := exec.LookPath("claude"); err == nil {
		return p, nil
	}
	for _, c := range []string{
		filepath.Join(os.Getenv("USERPROFILE"), `.local\bin\claude.exe`),
		filepath.Join(os.Getenv("APPDATA"), `npm\claude.cmd`),
	} {
		if exists(c) {
			return c, nil
		}
	}
	return "", errors.New("Claude Code was not found; install it (see https://docs.claude.com/en/docs/claude-code/setup)")
}

func (a *app) launchDesktop(r *Record) error {
	claude, err := findClaudeDesktop()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(r.Folder, 0o755); err != nil {
		return err
	}
	if err := startDetached(exec.Command(claude, "--user-data-dir="+r.Folder), false); err != nil {
		return err
	}
	fmt.Printf("Started Claude Desktop as %s\n", r.Label)
	return a.touch(r)
}

func (a *app) launchMain() error {
	claude, err := findClaudeDesktop()
	if err != nil {
		return err
	}
	if err := startDetached(exec.Command(claude), false); err != nil {
		return err
	}
	fmt.Println("Started your main Claude Desktop")
	return nil
}

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
		workDir = os.Getenv("USERPROFILE")
	}
	if workDir, err = filepath.Abs(workDir); err != nil {
		return err
	}
	if info, err := os.Stat(workDir); err != nil || !info.IsDir() {
		return fmt.Errorf("folder not found: %s", workDir)
	}

	// Windows Terminal treats ';' as a command separator.
	title := strings.ReplaceAll("Claude Code - "+r.Label, ";", ",")
	script := fmt.Sprintf("$Host.UI.RawUI.WindowTitle = %s; $env:CLAUDE_CONFIG_DIR = %s; Set-Location -LiteralPath %s; & %s",
		psQuote(title), psQuote(configDir), psQuote(workDir), psQuote(claude))
	psArgs := []string{"-NoExit", "-NoProfile", "-EncodedCommand", encodePS(script)}

	var cmd *exec.Cmd
	newConsole := false
	if wt, err := exec.LookPath("wt.exe"); err == nil {
		cmd = exec.Command(wt, append([]string{"new-tab", "--title", title, "powershell.exe"}, psArgs...)...)
	} else {
		cmd = exec.Command("powershell.exe", psArgs...)
		newConsole = true
	}
	if err := startDetached(cmd, newConsole); err != nil {
		return err
	}
	fmt.Printf("Started Claude Code as %s in %s\n", r.Label, workDir)
	return a.touch(r)
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
		fmt.Printf("  %2d)  %-32s %s\n", i+1, r.Label, when)
	}
	fmt.Println()
	fmt.Println("   1, 2, ...   Claude Desktop as that user")
	fmt.Println("  C1, C2, ...  Claude Code as that user (opens a terminal)")
	fmt.Println()
	fmt.Println("   M)  Main Claude Desktop (your normal profile)")
	fmt.Println("   A)  Add a new user")
	fmt.Println("   R)  Remove a user")
	fmt.Println("   Q)  Quit")
	fmt.Println()
}

func (a *app) pickCode(i int) (bool, error) {
	if i < 0 || i >= len(a.records) {
		fmt.Println("No such number.")
		time.Sleep(time.Second)
		return false, nil
	}
	dir := strings.Trim(a.prompt("Project folder for Claude Code (blank = your home folder): "), `"`)
	return true, a.launchCode(a.records[i], dir)
}

func safeFolderName(label string) string {
	s := strings.Trim(unsafeChars.ReplaceAllString(strings.TrimSpace(label), "_"), "._-")
	if s == "" {
		return "profile"
	}
	return s
}

func (a *app) addProfile() (bool, error) {
	label := a.prompt("Email or name for the new user (blank to cancel): ")
	if label == "" {
		return false, nil
	}
	if a.find(label) != nil {
		fmt.Printf("A user named %s already exists.\n", label)
		time.Sleep(2 * time.Second)
		return false, nil
	}
	safe := safeFolderName(label)
	folder := filepath.Join(a.dataRoot, safe)
	for n := 2; exists(folder); n++ {
		folder = filepath.Join(a.dataRoot, fmt.Sprintf("%s-%d", safe, n))
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return false, err
	}
	r := &Record{Label: label, Folder: folder, Created: time.Now().Format("2006-01-02")}
	a.records = append(a.records, r)
	if err := a.save(); err != nil {
		return false, err
	}
	fmt.Printf("Added %s  ->  %s\n", label, folder)

	switch ans := strings.ToLower(a.prompt("Start it now and sign in? [D]esktop / [C]ode / [N]o (default D): ")); {
	case ans == "" || strings.HasPrefix(ans, "d") || strings.HasPrefix(ans, "y"):
		return true, a.launchDesktop(r)
	case strings.HasPrefix(ans, "c"):
		return a.pickCode(len(a.records) - 1)
	}
	return false, nil
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
	fmt.Println("That signs the user out on this PC and cannot be undone. Conversations stay on the Claude account.")
	if a.prompt("Type DELETE to confirm, anything else to cancel: ") != "DELETE" {
		fmt.Println("Cancelled.")
		return nil
	}
	for _, d := range []string{r.Folder, codeDir} {
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}
	a.records = append(a.records[:n-1], a.records[n:]...)
	if err := a.save(); err != nil {
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

func clearScreen() {
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}
