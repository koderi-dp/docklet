package runspec

import (
	"fmt"
	"net/url"
	"strings"
)

var dbKinds = []dbKind{
	{"postgres", "PostgreSQL", []string{"postgres", "postgis", "timescale", "pgvector"}, []int{5432}, postgresCreds, postgresEntries},
	{"mysql", "MySQL / MariaDB", []string{"mysql", "mariadb"}, []int{3306}, mysqlCreds, mysqlEntries},
	{"mongodb", "MongoDB", []string{"mongo"}, []int{27017}, mongoCreds, mongoEntries},
	{"redis", "Redis / Valkey", []string{"redis", "valkey"}, []int{6379}, redisCreds, redisEntries},
	{"mssql", "Microsoft SQL Server", []string{"mssql", "sql-server"}, []int{1433}, mssqlCreds, mssqlEntries},
	{"rabbitmq", "RabbitMQ", []string{"rabbitmq"}, []int{5672}, rabbitCreds, rabbitEntries},
	{"clickhouse", "ClickHouse", []string{"clickhouse"}, []int{8123, 9000}, clickhouseCreds, clickhouseEntries},
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func dbPath(db string) string {
	if db == "" {
		return ""
	}
	return "/" + url.PathEscape(db)
}

func postgresCreds(b *connBuilder) {
	_, b.user = b.env("POSTGRES_USER", "POSTGRESQL_USERNAME")
	b.user = orDefault(b.user, "postgres")
	_, b.db = b.env("POSTGRES_DB", "POSTGRESQL_DATABASE")
	b.db = orDefault(b.db, b.user)
	b.setPassword("POSTGRES_PASSWORD", "POSTGRESQL_PASSWORD")
	b.alwaysPassword = true
}

func postgresEntries(b *connBuilder, _ int) {
	b.add("URL", fmt.Sprintf("postgresql://%s%s%s", b.userinfo(), b.addr(), dbPath(b.db)), true)
	b.add("psql", fmt.Sprintf("psql -h %s -p %d -U %s %s", b.host, b.port, Bash.Quote(b.user), Bash.Quote(b.db)), false)
}

func mysqlCreds(b *connBuilder) {
	_, b.user = b.env("MYSQL_USER", "MARIADB_USER")
	if b.user != "" {
		b.setPassword("MYSQL_PASSWORD", "MARIADB_PASSWORD")
	} else {
		b.user = "root"
		b.setPassword("MYSQL_ROOT_PASSWORD", "MARIADB_ROOT_PASSWORD")
	}
	_, b.db = b.env("MYSQL_DATABASE", "MARIADB_DATABASE")
	b.alwaysPassword = true
}

func mysqlEntries(b *connBuilder, _ int) {
	b.add("URL", fmt.Sprintf("mysql://%s%s%s", b.userinfo(), b.addr(), dbPath(b.db)), true)
	cmd := fmt.Sprintf("mysql -h %s -P %d -u %s -p", b.host, b.port, Bash.Quote(b.user))
	if b.db != "" {
		cmd += " " + Bash.Quote(b.db)
	}
	b.add("mysql", cmd, false)
}

func mongoCreds(b *connBuilder) {
	_, b.user = b.env("MONGO_INITDB_ROOT_USERNAME")
	b.setPassword("MONGO_INITDB_ROOT_PASSWORD")
}

func mongoEntries(b *connBuilder, _ int) {
	q := ""
	if b.user != "" {
		q = "/?authSource=admin"
	}
	b.add("URL", fmt.Sprintf("mongodb://%s%s%s", b.userinfo(), b.addr(), q), true)
}

func redisCreds(b *connBuilder) { b.setPassword("REDIS_PASSWORD", "VALKEY_PASSWORD") }

func redisEntries(b *connBuilder, _ int) {
	auth := ""
	if b.hasPassword() {
		p, _ := b.password()
		auth = ":" + p + "@"
	}
	u := fmt.Sprintf("redis://%s%s", auth, b.addr())
	b.add("URL", u, true)
	if b.hasPassword() {
		b.add("redis-cli", "redis-cli -u "+Bash.Quote(u), true)
	} else {
		b.add("redis-cli", fmt.Sprintf("redis-cli -h %s -p %d", b.host, b.port), false)
	}
}

func mssqlCreds(b *connBuilder) {
	b.user = "sa"
	b.setPassword("MSSQL_SA_PASSWORD", "SA_PASSWORD")
	b.alwaysPassword = true
}

func mssqlEntries(b *connBuilder, _ int) {
	b.add("URL", fmt.Sprintf("sqlserver://%s%s", b.userinfo(), b.addr()), true)
	b.add("sqlcmd", fmt.Sprintf("sqlcmd -S %s,%d -U %s", b.host, b.port, b.user), false)
}

func rabbitCreds(b *connBuilder) {
	_, b.user = b.env("RABBITMQ_DEFAULT_USER")
	b.user = orDefault(b.user, "guest")
	b.setPassword("RABBITMQ_DEFAULT_PASS")
	_, b.vhost = b.env("RABBITMQ_DEFAULT_VHOST")
	b.alwaysPassword = true
}

func rabbitEntries(b *connBuilder, _ int) {
	vhost := "/"
	if b.vhost != "" && b.vhost != "/" {
		vhost = "/" + url.PathEscape(b.vhost)
	}
	b.add("URL", fmt.Sprintf("amqp://%s%s%s", b.userinfo(), b.addr(), vhost), true)
}

func clickhouseCreds(b *connBuilder) {
	_, b.user = b.env("CLICKHOUSE_USER")
	b.user = orDefault(b.user, "default")
	_, b.db = b.env("CLICKHOUSE_DB")
	b.setPassword("CLICKHOUSE_PASSWORD")
}

func clickhouseEntries(b *connBuilder, containerPort int) {
	if containerPort == 8123 {
		q := ""
		if b.db != "" {
			q = "/?database=" + url.QueryEscape(b.db)
		}
		b.add("HTTP URL", fmt.Sprintf("http://%s%s%s", b.userinfo(), b.addr(), q), true)
		return
	}
	b.add("Native URL", fmt.Sprintf("clickhouse://%s%s%s", b.userinfo(), b.addr(), dbPath(b.db)), true)
	cmd := fmt.Sprintf("clickhouse-client --host %s --port %d --user %s", b.host, b.port, Bash.Quote(b.user))
	if b.hasPassword() {
		cmd += " --password"
	}
	if b.db != "" {
		cmd += " --database " + Bash.Quote(b.db)
	}
	b.add("clickhouse-client", strings.TrimSpace(cmd), false)
}
