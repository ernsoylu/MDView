package parser

import "testing"

func TestLineOf(t *testing.T) {
	d := Parse([]byte("line one\nline two\nline three\n"))
	cases := []struct{ offset, want int }{
		{0, 1},  // start of line one
		{8, 1},  // the newline still belongs to line one
		{9, 2},  // start of line two
		{17, 2}, // end of line two
		{18, 3}, // start of line three
	}
	for _, tc := range cases {
		if got := d.LineOf(tc.offset); got != tc.want {
			t.Errorf("LineOf(%d) = %d, want %d", tc.offset, got, tc.want)
		}
	}
}
