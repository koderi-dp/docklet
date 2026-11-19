package compose

import (
	"bufio"
	"slices"
	"strings"
	"testing"
)

func scanLines(t *testing.T, in string) []string {
	t.Helper()
	sc := bufio.NewScanner(strings.NewReader(in))
	sc.Split(splitLines)
	var out []string
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSplitLines(t *testing.T) {
	for _, tc := range []struct {
		name, in string
		want     []string
	}{
		{"empty input yields nothing", "", nil},
		{"LF", "a\nb\n", []string{"a", "b"}},
		{"CRLF counts as one separator", "a\r\nb\r\n", []string{"a", "b"}},
		{"bare CR redraws are separate lines", "10%\r50%\r100%\n", []string{"10%", "50%", "100%"}},
		{"no trailing newline keeps the last line", "a\nb", []string{"a", "b"}},
		{"blank lines are preserved for the caller to drop", "a\n\nb\n", []string{"a", "", "b"}},
		{"CRLF followed by CR", "a\r\n\rb", []string{"a", "", "b"}},
		{"only a newline is one empty line", "\n", []string{""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := scanLines(t, tc.in); !slices.Equal(got, tc.want) {
				t.Fatalf("%q -> %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// Without more data (not at EOF) a line break at the very end of the buffer still ends the line.
func TestSplitLines_TokenBoundaries(t *testing.T) {
	adv, tok, err := splitLines([]byte("a\r\nb"), false)
	if err != nil || adv != 3 || string(tok) != "a" {
		t.Fatalf("CRLF must be consumed as one separator: adv=%d tok=%q err=%v", adv, tok, err)
	}
	adv, tok, _ = splitLines([]byte("partial"), false)
	if adv != 0 || tok != nil {
		t.Fatalf("an unfinished line must wait for more data: adv=%d tok=%q", adv, tok)
	}
	adv, tok, _ = splitLines([]byte("tail"), true)
	if adv != 4 || string(tok) != "tail" {
		t.Fatalf("at EOF the rest is the last line: adv=%d tok=%q", adv, tok)
	}
}
