package ui

import "strings"

// fuzzyMatch reports whether query matches s as a case-insensitive
// subsequence, with a score for ranking: consecutive runs and word starts
// are rewarded, longer targets mildly penalized. Empty queries match
// everything with score 0, which keeps document order in the TOC.
func fuzzyMatch(query, s string) (score int, ok bool) {
	if query == "" {
		return 0, true
	}
	q := []rune(strings.ToLower(query))
	t := []rune(strings.ToLower(s))
	qi := 0
	prev := -2
	for ti := 0; ti < len(t) && qi < len(q); ti++ {
		if t[ti] != q[qi] {
			continue
		}
		score++
		if ti == prev+1 {
			score += 3 // consecutive run
		}
		if ti == 0 || t[ti-1] == ' ' {
			score += 2 // word start
		}
		prev = ti
		qi++
	}
	if qi < len(q) {
		return 0, false
	}
	return score - (len(t)-len(q))/4, true
}
