package services

import (
	"strings"
	"testing"

	"docklet/internal/model"
)

func TestMaskComposeConfig_HidesSecretValuesAndKeepsStructure(t *testing.T) {
	in := strings.Join([]string{
		"services:",
		"  db:",
		"    environment:",
		"      POSTGRES_PASSWORD: hunter2",
		"      POSTGRES_DB: app",
		"      API_TOKEN: abc",
		"    volumes:",
		"      - ./keys:/keys",
		"    command:",
		"      - DB_SECRET=zzz",
		"      - PLAIN=ok",
		"    secrets:",
		"      - db_pass",
		"  web:",
		"    environment:",
		"      DATABASE_URL: postgres://u:pw123@db:5432/app",
		"secrets:",
		"  db_pass:",
		"    file: ./db.txt",
	}, "\n")
	out := maskComposeConfig(in)
	for _, leak := range []string{"hunter2", "abc", "zzz", "pw123"} {
		if strings.Contains(out, leak) {
			t.Errorf("leaked %q:\n%s", leak, out)
		}
	}
	for _, keep := range []string{"POSTGRES_DB: app", "- ./keys:/keys", "- PLAIN=ok", "secrets:", "file: ./db.txt", "- db_pass"} {
		if !strings.Contains(out, keep) {
			t.Errorf("lost %q:\n%s", keep, out)
		}
	}
}

func TestMergeProjectLogs_OrdersByTimestampAndTails(t *testing.T) {
	in := []projectLogChunk{
		{Service: "web", Container: "w1", Lines: []model.LogLine{{Timestamp: "2024-01-01T00:00:01Z", Text: "a"}, {Timestamp: "2024-01-01T00:00:03Z", Text: "c"}}},
		{Service: "db", Container: "d1", Lines: []model.LogLine{{Timestamp: "2024-01-01T00:00:02Z", Text: "b"}}},
	}
	got := mergeProjectLogs(in, 0)
	if len(got) != 3 || got[0].Text != "a" || got[1].Text != "b" || got[1].Service != "db" || got[2].Text != "c" {
		t.Fatalf("%+v", got)
	}
	if t2 := mergeProjectLogs(in, 2); len(t2) != 2 || t2[0].Text != "b" {
		t.Fatalf("tail: %+v", t2)
	}
}
