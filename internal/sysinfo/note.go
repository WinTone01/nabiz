// Package sysinfo reads the state of the tools that sit between this machine
// and the internet: bpftune's kernel auto-tuning and Unwall's DPI-bypass stack.
package sysinfo

import "github.com/WinTone01/nabiz/internal/i18n"

// Note is one observation about the local setup. Level is bad | warn | info | ok.
type Note struct {
	Level  string `json:"level"`
	Key    string `json:"key"`
	Text   string `json:"text"`
	Hint   string `json:"hint,omitempty"`
	Source string `json:"source,omitempty"`
}

// hintOf resolves an optional hint key, returning "" when the catalog has no
// entry so callers do not have to branch.
func hintOf(key string) string {
	if !i18n.Has(key) {
		return ""
	}
	return i18n.T(key)
}
