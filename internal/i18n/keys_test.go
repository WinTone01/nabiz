package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A missing catalog entry does not crash and does not warn: T returns the key
// itself, so the user reads "fnd.link-drops.title" in the report and nothing
// anywhere says why. Finding keys make this easy to hit, because they are built
// at runtime as "fnd." + key + ".title" and no compiler sees them.

// literalKey matches a translation key written out in full at a call site.
// The prefixes are the ones the catalogs actually use. "apply." and "monitor."
// are deliberately absent: they collide with the filenames apply.sh and
// monitor.jsonl, and no catalog key starts with them.
var literalKey = regexp.MustCompile(`"((?:fnd|adv|ui|err|cat)\.[a-zA-Z0-9._:-]+)"`)

// findingKey matches the short key given to findingList.add, which becomes
// "fnd.<key>.title" only at runtime. addText is excluded on purpose: it takes
// a title that has already been composed and looks up no such key.
var findingKey = regexp.MustCompile(`f\.add\(\s*(?:"[a-z]+"|level)\s*,\s*"([a-zA-Z0-9._:-]+)"`)

func TestEveryKeyUsedInCodeExists(t *testing.T) {
	root := repoRoot(t)
	var missing []string
	note := func(key, where string) {
		if key == "" || Has(key) {
			return
		}
		missing = append(missing, key+"  ("+where+")")
	}

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.HasPrefix(rel, "legacy") || strings.Contains(rel, "i18n/catalog") ||
			strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, match := range literalKey.FindAllStringSubmatch(string(body), -1) {
			note(match[1], rel)
		}
		for _, match := range findingKey.FindAllStringSubmatch(string(body), -1) {
			note("fnd."+match[1]+".title", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	if len(missing) > 0 {
		t.Errorf("%d translation keys are used in code but absent from the catalog:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// repoRoot walks up until it finds go.mod, so the test does not care where it
// is run from.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 6 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found above the test directory")
	return ""
}
