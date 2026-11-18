// eslint-disable-next-line no-control-regex
const ANSI_RE = /\u001b\[[0-9;?]*[ -/]*[@-~]/g;

/** Removes terminal colour/cursor escape sequences from a log line. */
export function stripAnsi(s: string): string {
  return s.replace(ANSI_RE, "");
}
