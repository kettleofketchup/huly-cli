package huly

import "testing"

func TestPriorityFromName(t *testing.T) {
	cases := map[string]Priority{
		"NoPriority": NoPriority, "Urgent": Urgent, "High": High,
		"Medium": Medium, "Low": Low,
		"urgent": Urgent, // case-insensitive
	}
	for name, want := range cases {
		got, ok := PriorityFromName(name)
		if !ok || got != want {
			t.Fatalf("PriorityFromName(%q) = %v,%v want %v", name, got, ok, want)
		}
	}
	if _, ok := PriorityFromName("bogus"); ok {
		t.Fatal("expected bogus priority to fail")
	}
}

func TestClassConstants(t *testing.T) {
	if ClassIssue != "tracker:class:Issue" {
		t.Fatalf("ClassIssue = %q", ClassIssue)
	}
	if IDNoParent != "tracker:ids:NoParent" {
		t.Fatalf("IDNoParent = %q", IDNoParent)
	}
	if SpaceTx != "core:space:Tx" {
		t.Fatalf("SpaceTx = %q", SpaceTx)
	}
}
