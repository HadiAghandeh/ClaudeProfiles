package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func TestSafeFolderName(t *testing.T) {
	cases := map[string]string{
		"hadi@example.com": "hadi_example.com",
		"  work account ":  "work_account",
		"...":              "profile",
		"a/b\\c":           "a_b_c",
	}
	for in, want := range cases {
		if got := safeFolderName(in); got != want {
			t.Errorf("safeFolderName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMakeShortcutsRejectsBadPlace(t *testing.T) {
	a := &app{}
	if err := a.makeShortcuts(&Record{Label: "x"}, "attic", false, ""); err == nil {
		t.Fatal("want an error for an unknown place")
	}
	if err := run(options{shortcut: "desktop"}); err == nil {
		t.Fatal("want an error for -shortcut without -profile")
	}
}

func TestPSQuote(t *testing.T) {
	if got := psQuote(`C:\it's here`); got != `'C:\it''s here'` {
		t.Errorf("psQuote = %s", got)
	}
}

func TestEncodePS(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(encodePS("é; x"))
	if err != nil {
		t.Fatal(err)
	}
	u := make([]uint16, len(raw)/2)
	for i := range u {
		u[i] = uint16(raw[2*i]) | uint16(raw[2*i+1])<<8
	}
	if got := string(utf16.Decode(u)); got != "é; x" {
		t.Errorf("round trip = %q", got)
	}
}

func TestCreateAndDeleteProfile(t *testing.T) {
	dir := t.TempDir()
	a := &app{
		dataRoot:     filepath.Join(dir, "Claude-Data"),
		codeDataRoot: filepath.Join(dir, "Claude-Code-Data"),
		recordsFile:  filepath.Join(dir, "profiles.json"),
	}
	r, err := a.createProfile("  test@example.com ")
	if err != nil {
		t.Fatal(err)
	}
	if r.Label != "test@example.com" || filepath.Base(r.Folder) != "test_example.com" || !exists(r.Folder) {
		t.Fatalf("created %+v", r)
	}
	if _, err := a.createProfile("TEST@example.com"); err != errExists {
		t.Fatalf("duplicate: err = %v, want errExists", err)
	}
	if _, err := a.createProfile("   "); err == nil {
		t.Fatal("blank label: want an error")
	}
	// A second profile whose safe name collides gets a numbered folder.
	r2, err := a.createProfile("test#example.com")
	if err != nil || filepath.Base(r2.Folder) != "test_example.com-2" {
		t.Fatalf("collision: %v %+v", err, r2)
	}

	if err := os.MkdirAll(a.codeFolder(r), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.deleteProfile(r); err != nil {
		t.Fatal(err)
	}
	if exists(r.Folder) || exists(a.codeFolder(r)) {
		t.Fatal("folders not deleted")
	}
	b := &app{recordsFile: a.recordsFile}
	if err := b.load(); err != nil || len(b.records) != 1 || b.records[0].Label != "test#example.com" {
		t.Fatalf("after delete: %v %+v", err, b.records)
	}
}

// profiles.json written by Windows PowerShell 5.1 has a UTF-8 BOM.
func TestLoadPowerShellFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "profiles.json")
	content := "\xef\xbb\xbf[\r\n    {\r\n        \"label\":  \"a@b.com\",\r\n        \"folder\":  \"C:\\\\x\\\\a_b.com\",\r\n        \"created\":  \"2026-11-16\",\r\n        \"lastUsed\":  \"\"\r\n    }\r\n]"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &app{recordsFile: file, codeDataRoot: `C:\code`}
	if err := a.load(); err != nil {
		t.Fatal(err)
	}
	if len(a.records) != 1 || a.records[0].Label != "a@b.com" || a.records[0].Folder != `C:\x\a_b.com` {
		t.Fatalf("records = %+v", a.records)
	}
	if err := a.save(); err != nil {
		t.Fatal(err)
	}
	b := &app{recordsFile: file}
	if err := b.load(); err != nil || len(b.records) != 1 {
		t.Fatalf("reload: %v %+v", err, b.records)
	}
}
