package runspec

import (
	"strings"
	"testing"
)

func pgInput() ConnInput {
	return ConnInput{Image: "postgres:16", Name: "db-1", Service: "db", Running: true,
		Env:       map[string]string{"POSTGRES_USER": "app", "POSTGRES_DB": "shop", "POSTGRES_PASSWORD": "p@ss word/1"},
		Exposed:   map[int]bool{5432: true},
		Published: []Port{{HostIP: "0.0.0.0", HostPort: "15432", ContainerPort: 5432, Proto: "tcp"}}}
}

func entries(info ConnInfo) map[string]string {
	m := map[string]string{}
	for _, e := range info.Entries {
		m[e.Label] = e.Text
	}
	return m
}

func TestConnectionStringPostgresPlaceholderByDefault(t *testing.T) {
	info, ok := ConnectionStrings(pgInput(), ConnContext{Type: "npipe"}, false)
	if !ok || !info.Reachable || info.Host != "localhost" {
		t.Fatalf("info %+v ok=%v", info, ok)
	}
	e := entries(info)
	if e["URL"] != "postgresql://app:<password>@localhost:15432/shop" {
		t.Errorf("URL %s", e["URL"])
	}
	if e["psql"] != "psql -h localhost -p 15432 -U app shop" {
		t.Errorf("psql %s", e["psql"])
	}
	if info.PasswordEnv != "POSTGRES_PASSWORD" {
		t.Errorf("password env %q", info.PasswordEnv)
	}
	for _, en := range info.Entries {
		if strings.Contains(en.Text, "p@ss") {
			t.Errorf("default entry leaks the password: %s", en.Text)
		}
	}
}

func TestConnectionStringPasswordEscapedWhenAsked(t *testing.T) {
	info, _ := ConnectionStrings(pgInput(), ConnContext{Type: "wsl"}, true)
	if got := entries(info)["URL"]; got != "postgresql://app:p%40ss%20word%2F1@localhost:15432/shop" {
		t.Errorf("URL %s", got)
	}
}

func TestConnectionStringNotPublished(t *testing.T) {
	in := pgInput()
	in.Published = nil
	info, ok := ConnectionStrings(in, ConnContext{Type: "npipe"}, false)
	if !ok || info.Reachable {
		t.Fatalf("info %+v", info)
	}
	if !strings.Contains(info.Note, "Not published: reachable only from other containers as db:5432") {
		t.Errorf("note %q", info.Note)
	}
	if got := entries(info)["URL"]; got != "postgresql://app:<password>@db:5432/shop" || !info.Entries[0].Internal {
		t.Errorf("internal entry: %s", got)
	}
}

func TestConnectionStringRemoteEngines(t *testing.T) {
	in := pgInput()
	info, _ := ConnectionStrings(in, ConnContext{Type: "tcp", Endpoint: "tcp://10.1.2.3:2376"}, false)
	if info.Host != "10.1.2.3" || !info.Reachable {
		t.Errorf("wildcard binding on a tcp engine should use the engine host: %+v", info)
	}
	in.Published[0].HostIP = "127.0.0.1"
	info, _ = ConnectionStrings(in, ConnContext{Type: "ssh", Endpoint: "ssh://me@box"}, false)
	if info.Reachable || !strings.Contains(info.Note, "not reachable from this machine") {
		t.Errorf("loopback on a remote engine must not be offered: %+v", info)
	}
}

func TestConnectionKindsAndExclusions(t *testing.T) {
	cases := []struct {
		image string
		port  int
		want  string
	}{
		{"postgres:16", 5432, "postgres"}, {"timescale/timescaledb:latest-pg16", 5432, "postgres"},
		{"pgvector/pgvector:pg16", 5432, "postgres"}, {"mariadb:11", 3306, "mysql"}, {"docker.io/library/mysql:8", 3306, "mysql"},
		{"mongo:7", 27017, "mongodb"}, {"redis:7-alpine", 6379, "redis"}, {"valkey/valkey:8", 6379, "redis"},
		{"mcr.microsoft.com/mssql/server:2022-latest", 1433, "mssql"}, {"rabbitmq:3-management", 5672, "rabbitmq"},
		{"clickhouse/clickhouse-server:24", 8123, "clickhouse"},
		{"mongo-express", 27017, ""}, {"prometheuscommunity/postgres-exporter", 9187, ""}, {"nginx", 80, ""},
		{"postgres:16", 80, ""}, // image looks right but the port is not there
	}
	for _, c := range cases {
		in := ConnInput{Image: c.image, Exposed: map[int]bool{c.port: true}, Env: map[string]string{}}
		if got := DetectDatabase(in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.image, got, c.want)
		}
	}
}

func TestConnectionCredentialsFromEnv(t *testing.T) {
	mysql := ConnInput{Image: "mysql:8", Name: "m", Exposed: map[int]bool{3306: true}, Running: true,
		Env:       map[string]string{"MYSQL_ROOT_PASSWORD": "rootpw", "MYSQL_DATABASE": "shop"},
		Published: []Port{{HostPort: "3306", ContainerPort: 3306, Proto: "tcp"}}}
	info, _ := ConnectionStrings(mysql, ConnContext{Type: "npipe"}, false)
	e := entries(info)
	if e["URL"] != "mysql://root:<password>@localhost:3306/shop" || e["mysql"] != "mysql -h localhost -P 3306 -u root -p shop" || info.PasswordEnv != "MYSQL_ROOT_PASSWORD" {
		t.Errorf("mysql: %v %s", e, info.PasswordEnv)
	}
	// No auth configured on Redis: no password invented.
	redis := ConnInput{Image: "redis:7", Name: "r", Exposed: map[int]bool{6379: true}, Running: true, Env: map[string]string{},
		Published: []Port{{HostPort: "6379", ContainerPort: 6379, Proto: "tcp"}}}
	info, _ = ConnectionStrings(redis, ConnContext{Type: "npipe"}, false)
	if got := entries(info)["URL"]; got != "redis://localhost:6379" {
		t.Errorf("redis %s", got)
	}
	// Mongo without root credentials in env: no credentials guessed.
	mongo := ConnInput{Image: "mongo:7", Name: "mg", Exposed: map[int]bool{27017: true}, Running: true, Env: map[string]string{},
		Published: []Port{{HostPort: "27017", ContainerPort: 27017, Proto: "tcp"}}}
	info, _ = ConnectionStrings(mongo, ConnContext{Type: "npipe"}, false)
	if got := entries(info)["URL"]; got != "mongodb://localhost:27017" {
		t.Errorf("mongo %s", got)
	}
}

func TestConnectHostCases(t *testing.T) {
	cases := []struct {
		ctx        ConnContext
		bind, host string
		reason     bool
	}{
		{ConnContext{Type: "npipe"}, "", "localhost", false},
		{ConnContext{Type: "wsl"}, "127.0.0.1", "localhost", false},
		{ConnContext{Type: "unix"}, "192.168.1.5", "", true},
		{ConnContext{Type: "tcp", Endpoint: "tcp://[fd00::1]:2376"}, "0.0.0.0", "fd00::1", false},
		{ConnContext{Type: "context", Endpoint: "ssh://u@h:22"}, "::", "h", false},
		{ConnContext{Type: "tcp", Endpoint: "tcp://h:2376"}, "10.0.0.1", "", true},
		{ConnContext{Type: "weird"}, "", "", true},
	}
	for _, c := range cases {
		host, reason := ConnectHost(c.ctx, c.bind)
		if host != c.host || (reason != "") != c.reason {
			t.Errorf("%+v bind=%q: host=%q reason=%q", c.ctx, c.bind, host, reason)
		}
	}
}
