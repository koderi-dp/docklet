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
