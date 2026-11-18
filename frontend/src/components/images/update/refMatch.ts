/** Canonical form of an image reference: default registry/namespace stripped, default tag added. */
export function canonicalRef(ref: string): string {
  let r = ref.trim().replace(/^(docker\.io|index\.docker\.io|registry-1\.docker\.io)\//, "");
  r = r.replace(/^library\//, "");
  const at = r.indexOf("@");
  if (at >= 0) return r;
  const lastSlash = r.lastIndexOf("/");
  return r.indexOf(":", lastSlash + 1) >= 0 ? r : `${r}:latest`;
}

/** True when two references name the same repository and tag. */
export function sameImageRef(a: string, b: string): boolean {
  return canonicalRef(a) === canonicalRef(b);
}
