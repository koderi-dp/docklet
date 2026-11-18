package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestSanitizeTar_RejectsZipSlipLinksAndDevices(t *testing.T) {
	bad := map[string][]tarItem{
		"traversal": {{name: "ok/", typ: tar.TypeDir}, {name: "ok/../../evil", typ: tar.TypeReg, body: "x"}},
		"absolute":  {{name: "/etc/passwd", typ: tar.TypeReg, body: "x"}},
		"hardlink":  {{name: "l", typ: tar.TypeLink, link: "../../etc/passwd"}},
		"device":    {{name: "dev", typ: tar.TypeChar}},
	}
	for name, items := range bad {
		if err := sanitizeTar(bytes.NewReader(makeTar(t, items)), io.Discard, nil); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
	good := makeTar(t, []tarItem{{name: "./", typ: tar.TypeDir}, {name: "./a/b.txt", typ: tar.TypeReg, body: "hi"}, {name: "ln", typ: tar.TypeSymlink, link: "a/b.txt"}})
	var out bytes.Buffer
	if err := sanitizeTar(bytes.NewReader(good), &out, nil); err != nil {
		t.Fatal(err)
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write(good)
	_ = zw.Close()
	var out2 bytes.Buffer
	if err := sanitizeTar(&gz, &out2, nil); err != nil || out2.Len() == 0 {
		t.Fatalf("gzip input: %v", err)
	}
	if err := sanitizeTar(strings.NewReader("this is not a tar archive at all"), io.Discard, nil); err == nil {
		t.Fatal("garbage must be rejected")
	}
}

func TestCleanContainerPath_RejectsRelativeTraversalAndNul(t *testing.T) {
	for _, p := range []string{"", "  ", "etc/passwd", "./x", "/a/../b", "/a/..", "/a\x00b", ".."} {
		if _, err := cleanContainerPath(p, true); err == nil {
			t.Errorf("%q should be rejected", p)
		}
	}
	if c, err := cleanContainerPath("/a//b/./c/", false); err != nil || c != "/a/b/c" {
		t.Fatalf("c=%q err=%v", c, err)
	}
	if _, err := cleanContainerPath("/", false); err == nil {
		t.Fatal("root must be rejected when not allowed")
	}
	if c, err := cleanContainerPath("/", true); err != nil || c != "/" {
		t.Fatalf("root allowed: %q %v", c, err)
	}
}

func TestDelete_RefusesRootTraversalAndStoppedContainers(t *testing.T) {
	argv, err := deleteArgv("/var/tmp/../x") // traversal rejected
	if err == nil {
		t.Fatalf("traversal accepted: %v", argv)
	}
	argv, err = deleteArgv("/var//log/app/")
	if err != nil || !reflect.DeepEqual(argv, []string{"rm", "-rf", "--", "/var/log/app"}) {
		t.Fatalf("argv=%v err=%v", argv, err)
	}
	for _, p := range []string{"/", "", "relative", "/a\x00"} {
		if _, err := deleteArgv(p); err == nil {
			t.Errorf("%q must be refused", p)
		}
	}

	f := &fakeFilesAPI{running: false}
	s := newFilesSvc(f)
	if err := s.Delete(context.Background(), "c1", "id", "/tmp/x"); err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("stopped container: %v", err)
	}
	if len(f.execArgv) != 0 {
		t.Fatal("must not exec on a stopped container")
	}
	f.running = true
	if err := s.Delete(context.Background(), "c1", "id", "/tmp/x"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.execArgv[0], []string{"rm", "-rf", "--", "/tmp/x"}) {
		t.Fatalf("argv = %v", f.execArgv[0])
	}
	f.execExit = 127
	if err := s.Delete(context.Background(), "c1", "id", "/tmp/x"); err == nil || !strings.Contains(err.Error(), "no rm") {
		t.Fatalf("missing rm: %v", err)
	}
	f.execExit, f.execStderr = 1, "rm: can't remove '/tmp/x': Read-only file system"
	if err := s.Delete(context.Background(), "c1", "id", "/tmp/x"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "read-only") {
		t.Fatalf("rofs: %v", err)
	}
}

func TestValidNewName_RejectsSeparatorsAndDots(t *testing.T) {
	for _, n := range []string{"", ".", "..", "a/b", "a\x00"} {
		if validNewName(n) == nil {
			t.Errorf("%q accepted", n)
		}
	}
}
