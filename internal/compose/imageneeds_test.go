package compose

import (
	"reflect"
	"testing"
)

const needsConfig = `{
  "name": "shop",
  "services": {
    "web":   {"image": "r.io/team/web:1", "depends_on": {"db": {"condition": "service_started"}}},
    "db":    {"image": "postgres:16", "depends_on": {"init": {"condition": "service_completed_successfully"}}},
    "init":  {"image": "r.io/team/init:1", "pull_policy": "always"},
    "api":   {"image": "r.io/team/api:1", "build": {"context": "."}},
    "cache": {"image": "redis:7", "pull_policy": "never"},
    "noimg": {"build": {"context": "./x"}}
  }
}`

func TestParseImageNeeds_ListsServicesComposeWouldPullAndSkipsBuiltOnes(t *testing.T) {
	got, err := ParseImageNeeds(needsConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []ImageNeed{
		{Service: "cache", Image: "redis:7", PullPolicy: "never"},
		{Service: "db", Image: "postgres:16"},
		{Service: "init", Image: "r.io/team/init:1", PullPolicy: "always"},
		{Service: "web", Image: "r.io/team/web:1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
}

func TestParseImageNeeds_ASelectedServiceBringsItsDependencies(t *testing.T) {
	got, err := ParseImageNeeds(needsConfig, []string{"web"})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range got {
		names = append(names, n.Service)
	}
	if !reflect.DeepEqual(names, []string{"db", "init", "web"}) {
		t.Errorf("got %v", names)
	}
}

func TestParseImageNeeds_ReadsTheShortDependsOnList(t *testing.T) {
	got, err := ParseImageNeeds(`{"services":{"a":{"image":"a:1","depends_on":["b"]},"b":{"image":"b:1"},"c":{"image":"c:1"}}}`, []string{"a"})
	if err != nil || len(got) != 2 || got[0].Service != "a" || got[1].Service != "b" {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestParseImageNeeds_RejectsOutputThatIsNotTheConfig(t *testing.T) {
	if _, err := ParseImageNeeds("not json", nil); err == nil {
		t.Error("expected an error")
	}
}
