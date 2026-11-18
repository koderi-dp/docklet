import { describe, expect, it } from "vitest";
import { detectLevel } from "./logLevel";

const cases: Array<[string, string, ReturnType<typeof detectLevel>]> = [
  ["pino json (numeric)", '{"level":50,"time":1700000000000,"msg":"db connection lost"}', "error"],
  ["pino json info", '{"level":30,"time":1700000000000,"msg":"listening on 3000"}', "info"],
  ["json string level", '{"ts":"2024-05-01T10:00:00Z","level":"warn","msg":"slow query"}', "warn"],
  ["json severity (GCP)", '{"severity":"ERROR","message":"boom"}', "error"],
  ["json levelname (python-json-logger)", '{"levelname": "DEBUG", "message": "cache miss"}', "debug"],
  ["logfmt", 'time="2024-05-01T10:00:00Z" level=warning msg="disk almost full"', "warn"],
  ["logfmt unquoted info", "ts=2024-05-01T10:00:00Z caller=main.go:42 level=info msg=started", "info"],
  ["nginx error log", "2024/05/01 10:00:00 [error] 29#29: *1 connect() failed (111: Connection refused)", "error"],
  ["nginx notice", "2024/05/01 10:00:00 [notice] 1#1: start worker processes", "info"],
  ["spring boot", "2024-05-01 10:00:00.123  INFO 1 --- [           main] c.e.demo.Application : Started", "info"],
  ["java warn", "2024-05-01 10:00:00,123 WARN  [main] Pool exhausted", "warn"],
  ["python logging", "ERROR:root:Traceback follows", "error"],
  ["python warning", "WARNING:django.request:Not Found: /favicon.ico", "warn"],
  ["syslog priority", "<3>kernel: out of memory", "error"],
  ["syslog info priority", "<6>sshd[100]: Accepted publickey", "info"],
  ["glog", "E0501 10:00:00.000000       1 main.go:10] failed to start", "error"],
  ["logrus", 'ERRO[0000] cannot open file', "error"],
  ["rust env_logger", "[2024-05-01T10:00:00Z ERROR myapp::db] timeout", "error"],
  ["postgres", "2024-05-01 10:00:00.000 UTC [1] LOG:  database system is ready", "other"],
  ["postgres fatal", "2024-05-01 10:00:00.000 UTC [88] FATAL:  password authentication failed", "error"],
  ["leading word", "error: cannot find module 'x'", "error"],
  ["plain text", "Server started on port 8080", "other"],
  ["prose containing the word error is not an error line", "retrying after a transient error in the upstream", "other"],
  ["truncated json still found by logfmt-style key", '{"level":"error","msg":"cut off here', "error"],
  ["empty", "   ", "other"],
];

describe("detectLevel", () => {
  it.each(cases)("%s", (_name, line, want) => {
    expect(detectLevel(line)).toBe(want);
  });

  it("maps syslog numeric json levels below 10 and pino levels above", () => {
    expect(detectLevel('{"level":3,"msg":"x"}')).toBe("error");
    expect(detectLevel('{"level":6,"msg":"x"}')).toBe("info");
    expect(detectLevel('{"level":20,"msg":"x"}')).toBe("debug");
    expect(detectLevel('{"level":40,"msg":"x"}')).toBe("warn");
  });
});
