package compose

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"docklet/internal/model"
)

func outlineOf(t *testing.T, file string) model.ComposeFileOutline {
	t.Helper()
	b, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	return Outline(string(b))
}

func items(o model.ComposeFileOutline) []model.ComposeOutlineItem { return o.Services }

func TestOutlinePlain(t *testing.T) {
	o := Outline("name: x\nservices:\n  web:\n    image: a\n  db:\n    image: b\n")
	want := []model.ComposeOutlineItem{{Name: "web", Line: 3}, {Name: "db", Line: 5}}
	if o.Error != nil || !reflect.DeepEqual(items(o), want) {
		t.Fatalf("got %+v", o)
	}
}

func TestOutlineAnchors(t *testing.T) {
	o := outlineOf(t, "outline_anchors.yaml")
	want := []model.ComposeOutlineItem{{Name: "db", Line: 6}, {Name: "web", Line: 8}, {Name: "worker", Line: 11}, {Name: "alias", Line: 14}}
	if o.Error != nil || !reflect.DeepEqual(items(o), want) {
		t.Fatalf("got %+v", o)
	}
}

func TestOutlineMultiDoc(t *testing.T) {
	o := outlineOf(t, "outline_multidoc.yaml")
	want := []model.ComposeOutlineItem{{Name: "a", Line: 2}, {Name: "b", Line: 7}, {Name: "c", Line: 9}}
	if o.Error != nil || !reflect.DeepEqual(items(o), want) {
		t.Fatalf("got %+v", o)
	}
}

func TestOutlineEmptyResults(t *testing.T) {
	for name, text := range map[string]string{
		"no services key": "name: x\nvolumes: {}\n",
		"empty":           "",
		"list services":   "services:\n  - a\n  - b\n",
	} {
		o := Outline(text)
		if o.Error != nil || o.Services == nil || len(o.Services) != 0 {
			t.Errorf("%s: got %+v", name, o)
		}
	}
}

func TestOutlineBrokenYAML(t *testing.T) {
	cases := map[string]struct {
		text string
		line int
	}{
		"tab indent":     {"services:\n\tweb:\n    image: a\n", 2},
		"unclosed quote": {"a: 1\nb: 2\nc: \"open\n", 3},
	}
	for name, c := range cases {
		o := Outline(c.text)
		if o.Error == nil {
			t.Fatalf("%s: want an error", name)
		}
		if o.Error.Line != c.line {
			t.Errorf("%s: line %d, want %d (%q)", name, o.Error.Line, c.line, o.Error.Message)
		}
		if strings.HasPrefix(o.Error.Message, "yaml:") || o.Error.Message == "" {
			t.Errorf("%s: message %q", name, o.Error.Message)
		}
		if o.Services == nil || len(o.Services) != 0 {
			t.Errorf("%s: services %+v", name, o.Services)
		}
	}
}
