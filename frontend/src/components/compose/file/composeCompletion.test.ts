import { describe, expect, it } from "vitest";
import { CompletionContext, type CompletionResult } from "@codemirror/autocomplete";
import { EditorState } from "@codemirror/state";
import { composeSource } from "./composeCompletion";

/** Runs the source at the last "|" of doc. */
const run = (doc: string, images: string[] = [], explicit = false) => {
  const pos = doc.lastIndexOf("|");
  const state = EditorState.create({ doc: doc.slice(0, pos) + doc.slice(pos + 1) });
  return composeSource(() => images)(new CompletionContext(state, pos, explicit)) as CompletionResult | null;
};
const labels = (r: CompletionResult | null) => (r ? r.options.map((o) => o.label) : null);
const applied = (r: CompletionResult | null, label: string) => r?.options.find((o) => o.label === label)?.apply;

describe("composeSource", () => {
  it("offers service keys without the ones already present", () => {
    const l = labels(run("services:\n  web:\n    image: nginx\n    he|"))!;
    expect(l).toContain("healthcheck");
    expect(l).not.toContain("image");
  });

  it("replaces from the start of the typed word", () => expect(run("services:\n  web:\n    he|")!.from).toBe("services:\n  web:\n    ".length));

  it("inserts the key with a colon and a space", () => expect(applied(run("services:\n  web:\n    he|"), "healthcheck")).toBe("healthcheck: "));

  it("does not add a second colon", () => expect(applied(run("services:\n  web:\n    he|: {}\n"), "healthcheck")).toBe("healthcheck"));

  it("shows the schema's description and type", () => {
    const o = run("services:\n  web:\n    he|")!.options.find((x) => x.label === "healthcheck")!;
    expect(o.info).toBeTruthy();
    expect(o.type).toBe("property");
  });

  it("offers restart values", () => expect(labels(run("services:\n  web:\n    restart: |"))).toEqual(expect.arrayContaining(["unless-stopped"])));

  it("inserts a value as it is", () => expect(applied(run("services:\n  web:\n    restart: un|"), "unless-stopped")).toBeUndefined());

  it("offers images after image:", () => expect(labels(run("services:\n  web:\n    image: |", ["nginx:1.27"]))).toEqual(["nginx:1.27"]));

  it("shows a name too long for the list in full beside it", () => {
    const long = "992382845977.dkr.ecr.eu-central-1.amazonaws.com/studio/orchestration-api:v28.0.876.0";
    const options = run("services:\n  web:\n    image: |", [long, "nginx:1.27"])!.options;
    expect(options.find((o) => o.label === long)!.info).toBe(long);
    expect(options.find((o) => o.label === "nginx:1.27")!.info).toBeUndefined();
  });

  it("reads the image list on every call", () => {
    const images: string[] = [];
    const state = EditorState.create({ doc: "services:\n  web:\n    image: " });
    const source = composeSource(() => images);
    expect(source(new CompletionContext(state, state.doc.length, true))).toBeNull();
    images.push("redis:7");
    expect(labels(source(new CompletionContext(state, state.doc.length, true)) as CompletionResult)).toEqual(["redis:7"]);
  });

  it("offers other services under depends_on before anything else", () =>
    expect(labels(run("services:\n  web:\n    depends_on:\n      - |\n  db:\n    image: postgres"))![0]).toBe("db"));

  it("waits for a letter or Ctrl+Space on an empty key position", () => {
    expect(run("services:\n  web:\n    |")).toBeNull();
    expect(labels(run("services:\n  web:\n    |", [], true))).toContain("image");
  });

  it("stays silent in comments, strings and block scalars", () => {
    expect(run("services:\n  web: # he|", [], true)).toBeNull();
    expect(run('services:\n  web:\n    image: "ng|', ["nginx:1.27"], true)).toBeNull();
    expect(run("services:\n  web:\n    command: >\n      echo|", [], true)).toBeNull();
  });

  it("stays silent on an unknown parent, asked or not", () => {
    expect(run("services:\n  web:\n    x-custom:\n      a|")).toBeNull();
    expect(run("services:\n  web:\n    x-custom:\n      a|", [], true)).toBeNull();
  });
});
