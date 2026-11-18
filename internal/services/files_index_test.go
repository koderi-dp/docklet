package services

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
)

func idxTar(t *testing.T) []byte {
	return makeTar(t, []tarItem{
		{name: "./", typ: tar.TypeDir},
		{name: "./bin/sh", typ: tar.TypeReg, body: "x"}, // "bin" has no directory header
		{name: "./etc/", typ: tar.TypeDir},
		{name: "./etc/my file é.conf", typ: tar.TypeReg, body: "abc"},
		{name: "./etc/-dash", typ: tar.TypeReg},
		{name: "./usr/", typ: tar.TypeDir},
		{name: "./usr/lib/", typ: tar.TypeDir},
		{name: "./lib", typ: tar.TypeSymlink, link: "usr/lib"},            // relative link to a dir
		{name: "./usr/lib/self", typ: tar.TypeSymlink, link: "../../lib"}, // link back out through ".."
		{name: "./abs", typ: tar.TypeSymlink, link: "/etc"},
		{name: "./loopa", typ: tar.TypeSymlink, link: "loopb"},
		{name: "./loopb", typ: tar.TypeSymlink, link: "loopa"},
		{name: "./dangling", typ: tar.TypeSymlink, link: "nowhere"},
		{name: "./empty/", typ: tar.TypeDir},
	})
}

func buildIdx(t *testing.T, data []byte) *fileIndex {
	t.Helper()
	x, err := buildFileIndex(bg, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestIndex_ListsDirectoriesFirstAndInventsMissingDirs(t *testing.T) {
	x := buildIdx(t, idxTar(t))
	root, err := x.list("/")
	if err != nil || root.Method != "index" || root.IndexedEntries == 0 || root.IndexedAt != x.builtAt.UnixMilli() {
		t.Fatalf("root=%+v err=%v", root, err)
	}
	var got []string
	for _, e := range root.Entries {
		got = append(got, e.Name)
	}
	// directories (and links to directories) first, then the rest
	if !reflect.DeepEqual(got, []string{"abs", "bin", "empty", "etc", "lib", "usr", "dangling", "loopa", "loopb"}) {
		t.Fatalf("root names = %v", got)
	}
	bin, err := x.list("/bin")
	if err != nil || len(bin.Entries) != 1 || bin.Entries[0].Path != "/bin/sh" {
		t.Fatalf("implicit dir: %+v err=%v", bin, err)
	}
	etc, _ := x.list("/etc")
	if len(etc.Entries) != 2 || etc.Entries[0].Name != "-dash" || etc.Entries[1].Name != "my file é.conf" {
		t.Fatalf("etc=%+v", etc.Entries)
	}
	empty, err := x.list("/empty")
	if err != nil || empty.Entries == nil || len(empty.Entries) != 0 {
		t.Fatalf("empty=%#v err=%v", empty.Entries, err)
	}
	if _, err := x.list("/etc/-dash"); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("file listing: %v", err)
	}
	if _, err := x.list("/nope"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing: %v", err)
	}
}

func TestIndex_ResolvesSymlinksAndSurvivesLoops(t *testing.T) {
	x := buildIdx(t, idxTar(t))
	root, _ := x.list("/")
	by := map[string]int{}
	for i, e := range root.Entries {
		by[e.Name] = i
	}
	if lib := root.Entries[by["lib"]]; !lib.IsSymlink || !lib.IsDir || lib.LinkTarget != "usr/lib" {
		t.Fatalf("lib = %+v", lib)
	}
	if abs := root.Entries[by["abs"]]; !abs.IsDir || abs.LinkTarget != "/etc" {
		t.Fatalf("abs = %+v", abs)
	}
	for _, n := range []string{"loopa", "dangling"} {
		if e := root.Entries[by[n]]; e.IsDir || !e.IsSymlink {
			t.Fatalf("%s = %+v", n, e)
		}
	}
	l, err := x.list("/lib/self/self") // through three links
	if err != nil || len(l.Entries) != 1 || l.Entries[0].Name != "self" {
		t.Fatalf("through links: %+v %v", l.Entries, err)
	}
	if _, err := x.list("/loopa"); err == nil || !strings.Contains(err.Error(), "looping") {
		t.Fatalf("loop: %v", err)
	}
	if st, err := x.stat("/lib"); err != nil || !st.IsSymlink || st.LinkTarget != "usr/lib" {
		t.Fatalf("stat link = %+v %v", st, err)
	}
}

func TestIndexBuild_StopsAtEntryCapAndOnCancel(t *testing.T) {
	old := indexMaxEntries
	indexMaxEntries = 10
	defer func() { indexMaxEntries = old }()
	items := []tarItem{{name: "./", typ: tar.TypeDir}}
	for i := 0; i < 20; i++ {
		items = append(items, tarItem{name: "./f" + strconv.Itoa(i), typ: tar.TypeReg})
	}
	_, err := buildFileIndex(bg, bytes.NewReader(makeTar(t, items)))
	var unavailable errIndexUnavailable
	if err == nil || !asUnavailable(err, &unavailable) || !strings.Contains(err.Error(), "Start the container") {
		t.Fatalf("cap error = %v", err)
	}
	ctx, cancel := context.WithCancel(bg)
	cancel()
	indexMaxEntries = old
	many := []tarItem{{name: "./", typ: tar.TypeDir}}
	for i := 0; i < 2000; i++ {
		many = append(many, tarItem{name: "./f" + strconv.Itoa(i), typ: tar.TypeReg})
	}
	if _, err := buildFileIndex(ctx, bytes.NewReader(makeTar(t, many))); err == nil {
		t.Fatal("a cancelled context must stop the scan")
	}
}

func asUnavailable(err error, target *errIndexUnavailable) bool {
	e, ok := err.(errIndexUnavailable)
	if ok {
		*target = e
	}
	return ok
}

type slowReader struct{ ctx context.Context }

func (b slowReader) Read([]byte) (int, error) { <-b.ctx.Done(); return 0, b.ctx.Err() }

func TestIndexBuild_StopsAtItsDeadline(t *testing.T) {
	// a scan that never ends is cut by its deadline; getIndex turns that into errIndexUnavailable
	ctx, cancel := context.WithTimeout(bg, 30*time.Millisecond)
	defer cancel()
	if _, err := buildFileIndex(ctx, slowReader{ctx}); err == nil {
		t.Fatal("expected the scan to fail at its deadline")
	}
	if msg := (errIndexUnavailable{"scanning it took longer than 60 seconds"}).Error(); !strings.Contains(msg, "Start the container") {
		t.Fatalf("message = %q", msg)
	}
}

func stoppedFake(t *testing.T) *fakeFilesAPI {
	return &fakeFilesAPI{running: false, startedAt: "t1", copyFrom: idxTar(t), copyStat: container.PathStat{Mode: fs.ModeDir}}
}

func TestList_StoppedContainerScansOnceThenServesFromCache(t *testing.T) {
	f := stoppedFake(t)
	s := newFilesSvc(f)
	for _, p := range []string{"/", "/etc", "/lib/", "/empty", "/bin"} {
		l, err := s.List(bg, "c1", "id", p)
		if err != nil || l.Method != "index" || l.Entries == nil {
			t.Fatalf("%s: %+v %v", p, l, err)
		}
	}
	if f.copyFroms != 1 {
		t.Fatalf("archive scans = %d, want 1", f.copyFroms)
	}
	if len(f.execArgv) != 0 {
		t.Fatal("must not exec in a stopped container")
	}
	if st, err := s.Stat(bg, "c1", "id", "/etc/my file é.conf"); err != nil || st.Size != 3 {
		t.Fatalf("stat from index: %+v %v", st, err)
	}
	if f.copyFroms != 1 {
		t.Fatal("stat must be served from the index")
	}
}

func TestIndexCache_InvalidatesOnRestartWriteAndExpiry(t *testing.T) {
	f := stoppedFake(t)
	s := newFilesSvc(f)
	list := func() {
		t.Helper()
		if _, err := s.List(bg, "c1", "id", "/"); err != nil {
			t.Fatal(err)
		}
	}
	list()
	list()
	if f.copyFroms != 1 {
		t.Fatalf("scans = %d", f.copyFroms)
	}
	f.statErr = errors.New("no such file or directory") // Mkdir checks that the folder is new
	f.startedAt = "t2"                                  // the container was started again
	list()
	if f.copyFroms != 2 {
		t.Fatalf("stamp change must rescan, scans = %d", f.copyFroms)
	}
	if err := s.Mkdir(bg, "c1", "id", "/", "newdir"); err != nil {
		t.Fatal(err)
	}
	list()
	if f.copyFroms != 3 {
		t.Fatalf("a write must invalidate, scans = %d", f.copyFroms)
	}
	info, err := s.RefreshIndex(bg, "c1", "id")
	if err != nil || info.Entries == 0 || info.IndexedAt == 0 || f.copyFroms != 4 {
		t.Fatalf("refresh: %+v %v scans=%d", info, err, f.copyFroms)
	}
	if _, err := s.List(bg, "c1", "other", "/"); err != nil || f.copyFroms != 5 {
		t.Fatalf("other container has its own index: %v scans=%d", err, f.copyFroms)
	}
	s.idx.mu.Lock()
	s.idx.items[indexKey("c1", "id")].builtAt = time.Now().Add(-indexTTL - time.Second)
	s.idx.mu.Unlock()
	list()
	if f.copyFroms != 6 {
		t.Fatalf("an expired index must be rebuilt, scans = %d", f.copyFroms)
	}
}

func TestIndexCache_EvictsLeastRecentlyUsed(t *testing.T) {
	f := stoppedFake(t)
	s := newFilesSvc(f)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		if _, err := s.List(bg, "c1", id, "/"); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.idx.items) != indexCacheSize {
		t.Fatalf("cache holds %d", len(s.idx.items))
	}
	if s.idx.peek(indexKey("c1", "a"), "sha256:img|t1|fin") != nil {
		t.Fatal("the least recently used entry must be evicted")
	}
}

func TestParseLinkRecords_KeepsNewlinesInTargets(t *testing.T) {
	k, tg := parseLinkRecords([]byte("d\x1e/etc\x1f\nf\x1ea b\nc\x1f\n"))
	if !reflect.DeepEqual(k, []string{"d", "f"}) || !reflect.DeepEqual(tg, []string{"/etc", "a b\nc"}) {
		t.Fatalf("kinds=%v targets=%q", k, tg)
	}
}
