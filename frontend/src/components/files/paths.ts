/** POSIX path helpers for the file browser (container paths are always "/"-separated). */

export function parentPath(p: string): string {
  if (p === "/" || p === "") return "/";
  const i = p.replace(/\/+$/, "").lastIndexOf("/");
  return i <= 0 ? "/" : p.slice(0, i);
}

export function joinPath(dir: string, name: string): string {
  return dir === "/" ? `/${name}` : `${dir.replace(/\/+$/, "")}/${name}`;
}

export function baseName(p: string): string {
  const t = p.replace(/\/+$/, "");
  return t.slice(t.lastIndexOf("/") + 1) || "/";
}

/** Breadcrumb segments: [{label, path}] starting with the root. */
export function breadcrumbs(p: string, rootLabel = "/"): { label: string; path: string }[] {
  const out = [{ label: rootLabel, path: "/" }];
  let acc = "";
  for (const seg of p.split("/").filter(Boolean)) {
    acc += `/${seg}`;
    out.push({ label: seg, path: acc });
  }
  return out;
}

/** Normalises a typed path; returns null when it cannot be a valid absolute path. */
export function normalizeTyped(input: string): string | null {
  const t = input.trim();
  if (!t.startsWith("/") || t.includes("\0")) return null;
  const parts: string[] = [];
  for (const seg of t.split("/")) {
    if (seg === "" || seg === ".") continue;
    if (seg === "..") parts.pop();
    else parts.push(seg);
  }
  return `/${parts.join("/")}`;
}
