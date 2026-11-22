package compose

import (
	"bytes"
	"testing"
)

func TestSplitText(t *testing.T) {
	bom := "\xef\xbb\xbf"
	cases := []struct {
		name string
		in   string
		text string
		want TextStyle
	}{
		{"lf", "a: 1\nb: 2\n", "a: 1\nb: 2\n", TextStyle{}},
		{"crlf", "a: 1\r\nb: 2\r\n", "a: 1\nb: 2\n", TextStyle{CRLF: true}},
		{"mixed mostly crlf", "a\r\nb\r\nc\n", "a\nb\nc\n", TextStyle{CRLF: true}},
		{"mixed mostly lf", "a\nb\nc\r\n", "a\nb\nc\n", TextStyle{}},
		{"bom lf", bom + "a\nb\n", "a\nb\n", TextStyle{BOM: true}},
		{"bom crlf", bom + "a\r\nb\r\n", "a\nb\n", TextStyle{CRLF: true, BOM: true}},
		{"empty", "", "", TextStyle{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text, st, err := SplitText([]byte(c.in))
			if err != nil {
				t.Fatal(err)
			}
			if text != c.text || st != c.want {
				t.Fatalf("got %q %+v, want %q %+v", text, st, c.text, c.want)
			}
		})
	}
}

func TestJoinTextRoundTrip(t *testing.T) {
	bom := "\xef\xbb\xbf"
	for _, in := range []string{
		"a: 1\nb: 2\n", "a: 1\r\nb: 2\r\n",
		bom + "a: 1\nb: 2\n", bom + "a: 1\r\nb: 2\r\n",
		"no newline at end",
	} {
		text, st, err := SplitText([]byte(in))
		if err != nil {
			t.Fatal(err)
		}
		if got := JoinText(text, st); !bytes.Equal(got, []byte(in)) {
			t.Errorf("round trip of %q gave %q", in, got)
		}
	}
}

func TestSplitTextRejectsInvalidUTF8(t *testing.T) {
	if _, _, err := SplitText([]byte("a: \xff\xfe\n")); err == nil {
		t.Fatal("want an error for invalid UTF-8")
	}
}

func TestFileVersion(t *testing.T) {
	// sha256 of "abc"
	if got := FileVersion([]byte("abc")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("got %s", got)
	}
	if FileVersion([]byte("a\nb\n")) == FileVersion([]byte("a\r\nb\r\n")) {
		t.Fatal("LF and CRLF versions must differ")
	}
}
