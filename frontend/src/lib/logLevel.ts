// Tolerant log level detection for the formats containers commonly emit. Pure; returns "other"
// when no level can be told (the viewer still tints stderr lines).

export type LogLevel = "error" | "warn" | "info" | "debug" | "other";
export const LOG_LEVELS: readonly LogLevel[] = ["error", "warn", "info", "debug", "other"];

const NAMES: Record<string, LogLevel> = {
  fatal: "error", critical: "error", crit: "error", alert: "error", emerg: "error", emergency: "error", panic: "error",
  severe: "error", error: "error", err: "error", erro: "error", e: "error", f: "error",
  warn: "warn", warning: "warn", wrn: "warn", w: "warn",
  info: "info", inf: "info", information: "info", notice: "info", log: "info", i: "info",
  debug: "debug", dbg: "debug", debu: "debug", trace: "debug", trac: "debug", verbose: "debug", d: "debug", t: "debug",
};

function fromName(v: string): LogLevel | undefined {
  return NAMES[v.trim().toLowerCase()];
}

/** pino/bunyan use 10..60; syslog severities are 0..7 (0-3 error, 4 warn, 5-6 info, 7 debug). */
function fromNumber(n: number): LogLevel | undefined {
  if (!Number.isFinite(n)) return undefined;
  if (n >= 10) return n >= 50 ? "error" : n >= 40 ? "warn" : n >= 30 ? "info" : "debug";
  if (n < 0 || n > 7) return undefined;
  return n <= 3 ? "error" : n === 4 ? "warn" : n <= 6 ? "info" : "debug";
}

const JSON_KEYS = ["level", "severity", "lvl", "levelname", "loglevel", "log.level", "log_level"];

function fromJson(text: string): LogLevel | undefined {
  let obj: unknown;
  try {
    obj = JSON.parse(text);
  } catch {
    return undefined;
  }
  if (!obj || typeof obj !== "object") return undefined;
  const o = obj as Record<string, unknown>;
  const nested = (o.log as Record<string, unknown> | undefined)?.level;
  for (const k of JSON_KEYS) {
    const v = k === "log.level" ? (o[k] ?? nested) : o[k];
    if (typeof v === "string") {
      const l = fromName(v);
      if (l) return l;
    } else if (typeof v === "number") {
      const l = fromNumber(v);
      if (l) return l;
    }
  }
  return undefined;
}

const LOGFMT = /(?:^|[\s,{])"?(?:level|lvl|severity|loglevel)"?\s*[=:]\s*"?([A-Za-z]+)"?/i;
const BRACKET = /[[<(]\s*(fatal|critical|error|err|warning|warn|wrn|notice|info|inf|debug|dbg|trace)\s*[\]>)]/i;
const SYSLOG_PRI = /^<(\d{1,3})>/;
const GLOG = /^([IWEF])\d{4}\s/;
const LOGRUS = /^(ERRO|WARN|INFO|DEBU|TRAC)\[/;
const UPPER_TOKEN = /\b(FATAL|CRITICAL|ERROR|ERR|WARNING|WARN|WRN|NOTICE|INFO|INF|DEBUG|DBG|TRACE)\b/;
const LEADING_WORD = /^\s*(fatal|error|warning|warn|info|debug)\s*[:\]-]/i;

/** Detect the level of one log line (ANSI codes already stripped). */
export function detectLevel(text: string): LogLevel {
  const t = text.trim();
  if (!t) return "other";
  if (t.startsWith("{")) {
    const l = fromJson(t);
    if (l) return l;
  }
  const head = t.slice(0, 400);
  const pri = SYSLOG_PRI.exec(head);
  if (pri) {
    const l = fromNumber(Number(pri[1]) & 7);
    if (l) return l;
  }
  const g = GLOG.exec(head) ?? LOGRUS.exec(head);
  if (g) return fromName(g[1]) ?? "other";
  const kv = LOGFMT.exec(head);
  if (kv) {
    const l = fromName(kv[1]);
    if (l) return l;
  }
  const br = BRACKET.exec(head);
  if (br) return fromName(br[1]) ?? "other";
  const up = UPPER_TOKEN.exec(head.slice(0, 100));
  if (up) return fromName(up[1]) ?? "other";
  const lead = LEADING_WORD.exec(head);
  if (lead) return fromName(lead[1]) ?? "other";
  return "other";
}
