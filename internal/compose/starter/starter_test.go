package starter

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMinimal_StartsWithNameAndParses(t *testing.T) {
	text := Minimal("shop")
	if !strings.HasPrefix(text, "name: shop\n") {
		t.Fatalf("unexpected start:\n%s", text)
	}
	var doc struct {
		Name     string
		Services map[string]struct {
			Image string
			Ports []string
		}
	}
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatalf("invalid YAML: %v\n%s", err, text)
	}
	if doc.Name != "shop" || doc.Services["app"].Image != "nginx:alpine" || len(doc.Services["app"].Ports) != 1 {
		t.Fatalf("got %+v", doc)
	}
	if strings.Contains(text, "\r") {
		t.Fatal("the starter must use \n line endings")
	}
}

func TestTemplates_EachParsesAndStartsWithName(t *testing.T) {
	list := Templates()
	if len(list) != 3 {
		t.Fatalf("got %d templates", len(list))
	}
	for _, tpl := range list {
		if tpl.ID == "" || tpl.Title == "" {
			t.Fatalf("incomplete entry %+v", tpl)
		}
		text, err := Template(tpl.ID, "shop")
		if err != nil {
			t.Fatalf("%s: %v", tpl.ID, err)
		}
		if !strings.HasPrefix(text, "name: shop\n") || strings.Contains(text, "\r") {
			t.Errorf("%s: unexpected start or line endings:\n%s", tpl.ID, text)
		}
		var doc struct {
			Services map[string]struct {
				Environment map[string]string
			}
		}
		if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
			t.Fatalf("%s: invalid YAML: %v", tpl.ID, err)
		}
		if len(doc.Services) == 0 {
			t.Errorf("%s: no services", tpl.ID)
		}
		for svc, s := range doc.Services {
			for k, v := range s.Environment {
				if strings.HasSuffix(k, "PASSWORD") && !strings.HasPrefix(v, "${") {
					t.Errorf("%s: %s.%s holds a literal password", tpl.ID, svc, k)
				}
			}
		}
	}
}

func TestTemplate_UnknownID(t *testing.T) {
	if _, err := Template("nope", "shop"); err == nil {
		t.Fatal("want an error")
	}
}
