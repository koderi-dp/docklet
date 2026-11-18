package services

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
)

func TestListTarChildren_ReturnsDirectChildrenOnly(t *testing.T) {
	data := makeTar(t, []tarItem{
		{name: "usr/", typ: tar.TypeDir},
		{name: "usr/bin/", typ: tar.TypeDir},
		{name: "usr/bin/ls", typ: tar.TypeReg, body: "binary"},
		{name: "usr/readme.txt", typ: tar.TypeReg, body: "hello"},
		{name: "usr/sh", typ: tar.TypeSymlink, link: "bin/ls"},
		{name: "usr/../escape", typ: tar.TypeReg, body: "x"},
		{name: "other/file", typ: tar.TypeReg},
	})
	got, trunc, err := listTarChildren(bytes.NewReader(data), "/usr", 100, 1<<20)
	if err != nil || trunc {
		t.Fatalf("err=%v trunc=%v", err, trunc)
	}
	names := []string{}
	for _, e := range got {
		names = append(names, e.Name)
	}
	if !reflect.DeepEqual(names, []string{"bin", "readme.txt", "sh"}) {
		t.Fatalf("names = %v", names)
	}
	if !got[0].IsDir || got[1].Size != 5 || got[1].Path != "/usr/readme.txt" || !got[2].IsSymlink || got[2].LinkTarget != "bin/ls" {
		t.Fatalf("entries = %+v", got)
	}
}

func TestListTarChildren_HonoursEntryAndSizeCaps(t *testing.T) {
	items := []tarItem{{name: "./", typ: tar.TypeDir}}
	for _, n := range []string{"a", "b", "c", "d"} {
		items = append(items, tarItem{name: "./" + n, typ: tar.TypeReg})
	}
	data := makeTar(t, items)
	got, trunc, err := listTarChildren(bytes.NewReader(data), "/", 3, 1<<20)
	if err != nil || !trunc || len(got) != 3 || got[0].Path != "/a" {
		t.Fatalf("got=%+v trunc=%v err=%v", got, trunc, err)
	}
	big := makeTar(t, []tarItem{{name: "d/", typ: tar.TypeDir}, {name: "d/big", typ: tar.TypeReg, body: strings.Repeat("x", 4096)}})
	if _, _, err := listTarChildren(bytes.NewReader(big), "/d", 10, 1024); !errors.Is(err, errDirTooLarge) {
		t.Fatalf("want errDirTooLarge, got %v", err)
	}
	file := makeTar(t, []tarItem{{name: "f", typ: tar.TypeReg, body: "x"}})
	if _, _, err := listTarChildren(bytes.NewReader(file), "/f", 10, 1<<20); err == nil {
		t.Fatal("a file archive must not list as a directory")
	}
}

func TestParseStatLines_HandlesOddNamesAndModes(t *testing.T) {
	rec := func(mode, size, mt, name string) string {
		return mode + "\x1e" + size + "\x1e" + mt + "\x1e" + name + "\x1f\n"
	}
	out := []byte(rec("41ed", "4096", "1700000000", "bin") + rec("a1ff", "7", "1700000001", "sh") +
		rec("81a4", "12", "1700000002", "a|b.txt") + rec("81a4", "1", "1700000003", "two\nlines é") + "garbage\n")
	es := parseStatLines(out, "/usr")
	if len(es) != 4 {
		t.Fatalf("entries = %+v", es)
	}
	if !es[0].IsDir || es[0].Mode != "drwxr-xr-x" || es[0].ModTime != 1700000000000 {
		t.Errorf("dir = %+v", es[0])
	}
	if !es[1].IsSymlink || es[1].Size != 0 {
		t.Errorf("link = %+v", es[1])
	}
	if es[2].Name != "a|b.txt" || es[2].Size != 12 || es[2].Path != "/usr/a|b.txt" {
		t.Errorf("file = %+v", es[2])
	}
	if es[3].Name != "two\nlines é" {
		t.Errorf("newline/unicode name = %q", es[3].Name)
	}
	if modeFromRaw(0o104755)&fs.ModeSetuid == 0 {
		t.Error("setuid")
	}
	if got := parseStatLines(nil, "/x"); got == nil || len(got) != 0 {
		t.Errorf("empty output must give an empty non-nil slice, got %#v", got)
	}
}

func TestList_EmptyDirectorySerialisesAsEmptyArray(t *testing.T) {
	f := &fakeFilesAPI{running: true} // exec exits 0 with no output: an empty directory
	s := newFilesSvc(f)
	l, err := s.List(bg, "c1", "id", "/media")
	if err != nil || l.Method != "exec" {
		t.Fatalf("%+v %v", l, err)
	}
	b, _ := json.Marshal(l)
	if !strings.Contains(string(b), `"entries":[]`) {
		t.Fatalf("json = %s", b)
	}
}

func TestList_MapsStatExitCodesAndFallsBackToArchive(t *testing.T) {
	f := &fakeFilesAPI{running: true, execExit: 4}
	s := newFilesSvc(f)
	if _, err := s.List(bg, "c1", "id", "/gone"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing: %v", err)
	}
	f.execExit = 5
	if _, err := s.List(bg, "c1", "id", "/etc/passwd"); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("file: %v", err)
	}
	// no stat binary (127): fall back to the per-directory archive path
	f.execExit = 127
	f.copyFrom = makeTar(t, []tarItem{{name: "media/", typ: tar.TypeDir}, {name: "media/a", typ: tar.TypeReg}})
	f.copyStat = container.PathStat{Mode: fs.ModeDir}
	l, err := s.List(bg, "c1", "id", "/media")
	if err != nil || l.Method != "archive" || len(l.Entries) != 1 {
		t.Fatalf("fallback: %+v %v", l, err)
	}
}
