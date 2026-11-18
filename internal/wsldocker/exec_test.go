package wsldocker

import (
	"bufio"
	"bytes"
	"testing"
)

func TestCleanLine_RestoresWSLUTF16MessagesAndStripsTerminalCodes(t *testing.T) {
	utf16 := []byte{0xFF, 0xFE, 'N', 0, 'o', 0, ' ', 0, 'd', 0, 'i', 0, 's', 0, 't', 0, 'r', 0, 'o', 0}
	if got := cleanLine(utf16); got != "No distro" {
		t.Fatalf("utf16: %q", got)
	}
	if got := cleanLine([]byte("\x1b[1;32mdone\x1b[0m  ")); got != "done" {
		t.Fatalf("ansi: %q", got)
	}
}

func TestSplitLines_TreatsCarriageReturnProgressAsSeparateLines(t *testing.T) {
	sc := bufio.NewScanner(bytes.NewReader([]byte("10%\r50%\r100%\ndone")))
	sc.Split(splitLines)
	var got []string
	for sc.Scan() {
		got = append(got, sc.Text())
	}
	if len(got) != 4 || got[2] != "100%" || got[3] != "done" {
		t.Fatalf("%q", got)
	}
}
