//go:build windows

package main

import "testing"

func TestShortcutName(t *testing.T) {
	cases := []struct {
		label string
		code  bool
		want  string
	}{
		{"work@company.com", false, "Claude - work@company.com.lnk"},
		{"work@company.com", true, "Claude Code - work@company.com.lnk"},
		{` a/b:c*d?"e" `, false, "Claude - a_b_c_d__e_.lnk"},
	}
	for _, c := range cases {
		if got := shortcutName(c.label, c.code); got != c.want {
			t.Errorf("shortcutName(%q, %v) = %q, want %q", c.label, c.code, got, c.want)
		}
	}
}
