package services

import (
	"archive/tar"
	"context"
	"strings"
	"testing"
)

func TestIsBinaryContent(t *testing.T) {
	if isBinaryContent([]byte("plain text\nwith ünïcode\n")) {
		t.Error("text flagged binary")
	}
	if !isBinaryContent([]byte{0x7f, 'E', 'L', 'F', 0, 1}) {
		t.Error("NUL not detected")
	}
	if !isBinaryContent([]byte{0xff, 0xfe, 0xfd, 'a'}) {
		t.Error("invalid UTF-8 not detected")
	}
	cut := []byte("é")[:1] // truncated multibyte rune at the end
	if isBinaryContent(append([]byte("abc"), cut...)) {
		t.Error("cut rune at end must not count as binary")
	}
}

func TestReadText_RefusesBinaryAndTruncatesLargeText(t *testing.T) {
	f := &fakeFilesAPI{copyFrom: makeTar(t, []tarItem{{name: "prog", typ: tar.TypeReg, body: "ELF\x00\x01\x02"}})}
	_, err := readText(context.Background(), f, "id", "/bin/prog", "/bin/prog", 1000)
	if err == nil || !strings.Contains(err.Error(), "binary") {
		t.Fatalf("err = %v", err)
	}
	f.copyFrom = makeTar(t, []tarItem{{name: "a.txt", typ: tar.TypeReg, body: "hello world"}})
	txt, err := readText(context.Background(), f, "id", "/a.txt", "/a.txt", 5)
	if err != nil || txt.Content != "hello" || !txt.Truncated || txt.Size != 11 {
		t.Fatalf("txt=%+v err=%v", txt, err)
	}
}
