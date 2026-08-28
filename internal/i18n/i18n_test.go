package i18n

import (
	"strings"
	"testing"
)

// Every key must have Turkish text. English is the fallback, so a missing
// translation degrades gracefully rather than crashing - which is exactly why
// it needs a test: nothing else would ever notice.
func TestEveryKeyIsTranslated(t *testing.T) {
	if missing := MissingTranslations(); len(missing) > 0 {
		t.Errorf("%d keys have no Turkish text: %v", len(missing), missing)
	}
}

// A format verb count mismatch between the two languages is a crash waiting for
// whoever switches language at the wrong moment.
func TestFormatVerbsMatch(t *testing.T) {
	for _, key := range Keys() {
		english := In(EN, key)
		turkish := In(TR, key)
		if countVerbs(english) != countVerbs(turkish) {
			t.Errorf("%s: %d verbs in English, %d in Turkish\n  en: %s\n  tr: %s",
				key, countVerbs(english), countVerbs(turkish), english, turkish)
		}
	}
}

// countVerbs counts printf placeholders, ignoring the %% escape.
func countVerbs(text string) int {
	count := 0
	for index := 0; index < len(text); index++ {
		if text[index] != '%' {
			continue
		}
		if index+1 < len(text) && text[index+1] == '%' {
			index++
			continue
		}
		count++
	}
	return count
}

func TestDetectFallsBackToEnglish(t *testing.T) {
	for _, value := range []string{"", "de_DE.UTF-8", "C", "ja_JP"} {
		t.Setenv("NABIZ_LANG", "")
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", value)
		if got := Detect(); got != EN {
			t.Errorf("LANG=%q: expected English, got %s", value, got)
		}
	}
	for _, value := range []string{"tr_TR.UTF-8", "TR", "tr"} {
		t.Setenv("LANG", value)
		if got := Detect(); got != TR {
			t.Errorf("LANG=%q: expected Turkish, got %s", value, got)
		}
	}
	t.Setenv("LANG", "de_DE.UTF-8")
	t.Setenv("NABIZ_LANG", "tr")
	if got := Detect(); got != TR {
		t.Error("NABIZ_LANG should win over LANG")
	}
}

func TestUnknownKeyIsVisibleButHarmless(t *testing.T) {
	if got := T("no.such.key"); !strings.Contains(got, "no.such.key") {
		t.Errorf("unknown key should render as itself, got %q", got)
	}
}
