import { describe, expect, it } from "vitest";
import { cursorContext } from "./cursorPath";

/** A document with one cursor marker: the last "|" in it. */
function at(doc: string) {
  const offset = doc.lastIndexOf("|");
  return cursorContext(doc.slice(0, offset) + doc.slice(offset + 1), offset);
}

describe("cursorContext", () => {
  it("top level key", () => expect(at("services:\n  web:\n    image: nginx\nvol|")).toMatchObject({ path: [], kind: "key", prefix: "vol", siblings: ["services"] }));

  it("key inside a service", () =>
    expect(at("services:\n  web:\n    image: nginx\n    he|")).toMatchObject({ path: ["services", "web"], kind: "key", prefix: "he", siblings: ["image"] }));

  it("empty line takes its path from the cursor column", () =>
    expect(at("services:\n  web:\n    image: nginx\n    |")).toMatchObject({ path: ["services", "web"], kind: "key", prefix: "" }));

  it("nested map", () => expect(at("services:\n  web:\n    healthcheck:\n      in|")).toMatchObject({ path: ["services", "web", "healthcheck"], kind: "key" }));

  it("siblings below the cursor count too, other maps do not", () =>
    expect(at("services:\n  web:\n    image: nginx\n    |\n    ports:\n      - 80\n  db:\n    restart: always\n")).toMatchObject({
      path: ["services", "web"],
      siblings: ["image", "ports"],
    }));

  it("value after a key", () =>
    expect(at("services:\n  web:\n    restart: un|")).toMatchObject({ path: ["services", "web"], kind: "value", key: "restart", prefix: "un" }));

  it("value position right after the colon and space", () => {
    const c = at("services:\n  web:\n    image: |");
    expect(c).toMatchObject({ kind: "value", key: "image", prefix: "" });
    expect(c!.from).toBe("services:\n  web:\n    image: ".length);
  });

  it("plain list item", () => {
    const c = at("services:\n  web:\n    depends_on:\n      - d|");
    expect(c).toMatchObject({ path: ["services", "web", "depends_on", "[]"], kind: "value", prefix: "d" });
    expect(c!.key).toBeUndefined();
  });

  it("list items at the indentation of their key", () =>
    expect(at("services:\n  web:\n    depends_on:\n    - db\n    - |")).toMatchObject({ path: ["services", "web", "depends_on", "[]"], kind: "value" }));

  it("key inside a list item map", () =>
    expect(at("services:\n  web:\n    volumes:\n      - type: bind\n        so|")).toMatchObject({
      path: ["services", "web", "volumes", "[]"],
      kind: "key",
      prefix: "so",
      siblings: ["type"],
    }));

  it("value of a key on the list item line", () =>
    expect(at("services:\n  web:\n    volumes:\n      - type: b|")).toMatchObject({ path: ["services", "web", "volumes", "[]"], kind: "value", key: "type", prefix: "b" }));

  it("from is the start of the typed word", () => {
    const c = at("services:\n  we|");
    expect(c!.from).toBe("services:\n  ".length);
  });

  it.each([
    ["comment", "services:\n  web: # no|"],
    ["comment line", "services:\n  # he|"],
    ["double quotes", 'services:\n  web:\n    image: "ng|"'],
    ["single quotes", "services:\n  web:\n    image: 'ng|'"],
    ["flow list", 'services:\n  web:\n    ports: ["80|"]'],
    ["flow map", "services:\n  web: { ima| }"],
    ["block scalar", "services:\n  web:\n    command: |-\n      echo h|"],
    ["block scalar, one word", "services:\n  web:\n    command: >\n      echo|"],
    ["tab indentation", "services:\n\tweb:\n\t\tim|"],
    ["tab on a parent line", "services:\n\tweb:\n    im|"],
    ["under a key that has a value", "services:\n  web: nginx\n    im|"],
    ["several words where a key goes", "services:\n  web:\n    not a key|"],
  ])("no suggestions in a %s", (_n, doc) => expect(at(doc)).toBeNull());

  it("ignores carriage returns", () =>
    expect(at("services:\r\n  web:\r\n    image: nginx\r\n    im|")).toMatchObject({ path: ["services", "web"], prefix: "im", siblings: ["image"] }));

  it("offset 0 and an empty document", () => expect(at("|")).toMatchObject({ path: [], kind: "key", prefix: "", siblings: [] }));

  it("never throws on odd offsets", () => {
    expect(() => cursorContext("services:\n", 999)).not.toThrow();
    expect(() => cursorContext("services:\n", -5)).not.toThrow();
  });

  it("reads each line above the cursor at most once", () => {
    const lines = ["services:"];
    for (let i = 0; i < 1250; i++) lines.push(`  svc${i}:`, "    image: nginx", "    ports:", '      - "80:80"');
    lines.push("  last:", "    image: nginx", "    he");
    const text = lines.join("\n");
    let reads = 0;
    const c = cursorContext(text, text.length, (n) => {
      reads++;
      return lines[n - 1];
    });
    expect(lines.length).toBeGreaterThan(5000);
    expect(c).toMatchObject({ path: ["services", "last"], prefix: "he", siblings: ["image"] });
    expect(reads).toBeLessThan(5100);
  });
});
