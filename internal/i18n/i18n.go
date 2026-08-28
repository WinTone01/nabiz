// Package i18n resolves every user-facing string.
//
// English is the default and the fallback: a missing Turkish entry renders in
// English rather than showing a raw key, so a half-translated build is still
// usable. Language is detected once from the environment and can be switched at
// runtime - callers that cache rendered text must regenerate it after a switch.
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Lang is a supported interface language.
type Lang string

const (
	// EN is the default and the fallback for missing translations.
	EN Lang = "en"
	// TR is Turkish.
	TR Lang = "tr"
)

var (
	mu      sync.RWMutex
	current = EN
	catalog = map[string][2]string{} // key -> {en, tr}
)

// register merges a catalog chunk. Called from the catalog_*.go init functions.
func register(entries map[string][2]string) {
	mu.Lock()
	defer mu.Unlock()
	for key, value := range entries {
		catalog[key] = value
	}
}

// Detect reads the environment. NABIZ_LANG wins, then the usual locale
// variables. Anything that is not Turkish resolves to English.
func Detect() Lang {
	candidates := []string{
		os.Getenv("NABIZ_LANG"),
		os.Getenv("LC_ALL"),
		os.Getenv("LC_MESSAGES"),
		os.Getenv("LANG"),
	}
	for _, value := range candidates {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "tr") {
			return TR
		}
		// an explicit non-Turkish locale settles it; keep looking only when the
		// variable is one of the placeholder locales
		if value != "c" && value != "posix" && value != "c.utf-8" {
			return EN
		}
	}
	return EN
}

// Set switches the active language.
func Set(lang Lang) {
	mu.Lock()
	defer mu.Unlock()
	if lang == TR {
		current = TR
		return
	}
	current = EN
}

// SetFromString accepts "tr", "en", "auto" or an empty string.
func SetFromString(value string) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tr", "turkish", "türkçe", "turkce":
		Set(TR)
	case "en", "english":
		Set(EN)
	default:
		Set(Detect())
	}
}

// Current returns the active language.
func Current() Lang {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Toggle flips between the two languages and returns the new one.
func Toggle() Lang {
	if Current() == TR {
		Set(EN)
		return EN
	}
	Set(TR)
	return TR
}

// Name is the language's own name, for the UI switcher.
func Name(lang Lang) string {
	if lang == TR {
		return "Türkçe"
	}
	return "English"
}

// T renders a key in the active language, formatting args printf-style.
// An unknown key renders as the key itself so the gap is visible but harmless.
func T(key string, args ...any) string {
	mu.RLock()
	entry, ok := catalog[key]
	lang := current
	mu.RUnlock()
	if !ok {
		if len(args) > 0 {
			return fmt.Sprintf(key+" %v", args)
		}
		return key
	}
	text := entry[0]
	if lang == TR && entry[1] != "" {
		text = entry[1]
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

// In renders a key in a specific language regardless of the active one, which
// the report writer uses to pin a document to one language.
func In(lang Lang, key string, args ...any) string {
	mu.RLock()
	entry, ok := catalog[key]
	mu.RUnlock()
	if !ok {
		return key
	}
	text := entry[0]
	if lang == TR && entry[1] != "" {
		text = entry[1]
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

// Has reports whether a key exists, used by tests that guard against typos.
func Has(key string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, ok := catalog[key]
	return ok
}

// Keys lists every registered key, for the completeness test.
func Keys() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(catalog))
	for key := range catalog {
		out = append(out, key)
	}
	return out
}

// MissingTranslations lists keys that have no Turkish text yet.
func MissingTranslations() []string {
	mu.RLock()
	defer mu.RUnlock()
	var out []string
	for key, entry := range catalog {
		if entry[1] == "" {
			out = append(out, key)
		}
	}
	return out
}

func init() { Set(Detect()) }
