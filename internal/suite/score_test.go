package suite

import "testing"

// One fault, several symptoms, charged once.
//
// Summing every finding made the score a function of how many symptoms the tool
// knows how to emit. A single bad pair in a cable drops the link, fails gigabit
// negotiation and leaves the link at 100 Mbit; that used to cost 8+8+3 while an
// unrelated single fault cost 8.
func TestOneFaultIsChargedOnce(t *testing.T) {
	grouped := []Finding{
		{Level: "bad", Key: "link-drops"},
		{Level: "bad", Key: "link-downshift", Because: "link-drops"},
		{Level: "warn", Key: "link-speed", Because: "link-downshift"},
	}
	if got := findingPenalty(grouped); got != 8 {
		t.Errorf("three symptoms of one fault cost %.0f, want 8", got)
	}

	separate := []Finding{
		{Level: "bad", Key: "link-drops"},
		{Level: "bad", Key: "dns-hijack"},
		{Level: "warn", Key: "conntrack"},
	}
	if got := findingPenalty(separate); got != 19 {
		t.Errorf("three unrelated problems cost %.0f, want 19", got)
	}
}

// The most serious member of a group sets its price, so a red symptom is not
// discounted just because its cause is only a warning.
func TestGroupCostsItsWorstMember(t *testing.T) {
	items := []Finding{
		{Level: "warn", Key: "bufferbloat-bad"},
		{Level: "bad", Key: "qdisc-drops", Because: "bufferbloat-bad"},
	}
	if got := findingPenalty(items); got != 8 {
		t.Errorf("group cost %.0f, want the worst member's 8", got)
	}
}

// Findings must not be able to drive the score to zero on their own; past the
// cap the measured numbers should be doing the talking.
func TestFindingPenaltyIsBounded(t *testing.T) {
	var many []Finding
	for _, key := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		many = append(many, Finding{Level: "bad", Key: key})
	}
	if got := findingPenalty(many); got != maxFindingPenalty {
		t.Errorf("ten separate faults cost %.0f, want the cap %d", got, maxFindingPenalty)
	}
}

// A symptom whose cause did not fire in this run is its own problem: a link
// that never dropped but negotiated at 100 Mbit has a real fault to report.
func TestSymptomWithoutItsCauseStandsAlone(t *testing.T) {
	items := linkCauses([]Finding{{Level: "warn", Key: "link-speed"}})
	if items[0].Because != "" {
		t.Errorf("attached link-speed to an absent cause: %q", items[0].Because)
	}
	if got := findingPenalty(items); got != 3 {
		t.Errorf("cost %.0f, want 3", got)
	}
}

// A Because cycle must not hang the scorer.
func TestRootCauseSurvivesACycle(t *testing.T) {
	byKey := map[string]Finding{
		"a": {Key: "a", Because: "b"},
		"b": {Key: "b", Because: "a"},
	}
	if root := RootCause("a", byKey); root != "a" && root != "b" {
		t.Errorf("unexpected root %q", root)
	}
}
