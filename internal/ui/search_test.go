package ui

import "testing"

func TestFindMatchesSmartCase(t *testing.T) {
	plain := []string{"Alpha alpha", "no hit", "ALPHA"}

	got := findMatches(plain, "alpha")
	if len(got) != 3 {
		t.Fatalf("lowercase query: %d matches, want 3 (case-insensitive)", len(got))
	}
	if got[0] != (match{line: 0, start: 0, end: 5}) || got[1] != (match{line: 0, start: 6, end: 11}) {
		t.Errorf("unexpected offsets: %+v", got)
	}

	got = findMatches(plain, "Alpha")
	if len(got) != 1 || got[0].line != 0 {
		t.Fatalf("uppercase query: %+v, want exactly the exact-case hit", got)
	}
}

func TestFindMatchesEmptyAndAdjacent(t *testing.T) {
	if got := findMatches([]string{"aaa"}, ""); got != nil {
		t.Errorf("empty query matched: %+v", got)
	}
	// Non-overlapping adjacent hits.
	got := findMatches([]string{"aaaa"}, "aa")
	if len(got) != 2 {
		t.Errorf("adjacent: %d matches, want 2: %+v", len(got), got)
	}
}

func TestFuzzyMatchSubsequence(t *testing.T) {
	if _, ok := fuzzyMatch("abc", "a-b-c"); !ok {
		t.Error("scattered subsequence should match")
	}
	if _, ok := fuzzyMatch("abc", "acb"); ok {
		t.Error("out-of-order should not match")
	}
	if _, ok := fuzzyMatch("ABC", "abc"); !ok {
		t.Error("matching is case-insensitive")
	}
	if sc, ok := fuzzyMatch("", "anything"); !ok || sc != 0 {
		t.Errorf("empty query = (%d, %v), want (0, true)", sc, ok)
	}
}

func TestFuzzyMatchRanking(t *testing.T) {
	// Consecutive substring beats scattered letters.
	tight, _ := fuzzyMatch("sec", "second")
	scattered, ok := fuzzyMatch("sec", "search engine core")
	if !ok || tight <= scattered {
		t.Errorf("consecutive %d should outrank scattered %d", tight, scattered)
	}
	// Same match, shorter target wins.
	short, _ := fuzzyMatch("toc", "toc")
	long, _ := fuzzyMatch("toc", "toc and a lot of trailing words")
	if short <= long {
		t.Errorf("shorter target %d should outrank longer %d", short, long)
	}
}
