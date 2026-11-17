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
