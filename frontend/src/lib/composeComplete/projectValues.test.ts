import { describe, expect, it } from "vitest";
import { cursorContext } from "./cursorPath";
import { documentNames, imageNames, projectValues } from "./projectValues";

const tail = "\n  db:\n    image: postgres\nvolumes:\n  data:\n  cache: {}\nnetworks:\n  back:\n";

/** Suggestions for a `web` service whose body ends at the cursor ("|"); db, the volumes and the network follow it. */
function suggest(webBody: string, images: string[] = []) {
  const head = "services:\n  web:\n" + webBody;
  const offset = head.lastIndexOf("|");
  const text = head.slice(0, offset) + head.slice(offset + 1) + tail;
  const ctx = cursorContext(text, offset);
  if (!ctx) throw new Error("no cursor context");
  return projectValues(ctx, documentNames(text), images).map((s) => s.label);
}

describe("documentNames", () => {
  const doc = "services:\n  web:\n    image: nginx\n  db:\n    image: postgres\n# note\nvolumes:\n  data:\n  cache: {}\n\nnetworks:\n  back:\n    driver: bridge\n";
  it("reads names from the document", () => expect(documentNames(doc)).toEqual({ services: ["web", "db"], volumes: ["data", "cache"], networks: ["back"] }));
  it("reads names with Windows line endings", () =>
    expect(documentNames(doc.replace(/\n/g, "\r\n"))).toEqual({ services: ["web", "db"], volumes: ["data", "cache"], networks: ["back"] }));
  it("survives a document with no such maps or broken text", () => {
    expect(documentNames("serv")).toEqual({ services: [], volumes: [], networks: [] });
    expect(documentNames("")).toEqual({ services: [], volumes: [], networks: [] });
    expect(documentNames("services: [\n  web:\n")).toEqual({ services: [], volumes: [], networks: [] });
  });
});

describe("imageNames", () => {
  it("lists image names once and skips untagged ones", () =>
    expect(imageNames([["nginx:1.27", "nginx:latest"], ["<none>:<none>"], null, undefined, ["nginx:latest"], [], ["repo/app:<none>"]])).toEqual(["nginx:1.27", "nginx:latest"]));
});

describe("projectValues", () => {
  it("offers images after image:", () => expect(suggest("    image: |", ["nginx:1.27", "redis:7"])).toEqual(["nginx:1.27", "redis:7"]));

  it("offers other services under depends_on, list and map form", () => {
    expect(suggest("    depends_on:\n      - |")).toEqual(["db"]);
    expect(suggest("    depends_on:\n      |")).toEqual(["db"]);
  });

  it("offers other services for links, volumes_from and extends", () => {
    expect(suggest("    links:\n      - |")).toEqual(["db"]);
    expect(suggest("    volumes_from:\n      - |")).toEqual(["db"]);
    expect(suggest("    extends:\n      service: |")).toEqual(["db"]);
  });

  it("offers service: targets for network_mode", () => expect(suggest("    network_mode: |")).toEqual(["service:db"]));

  it("offers networks where a service references them, list and map form", () => {
    expect(suggest("    networks:\n      - |")).toEqual(["back"]);
    expect(suggest("    networks:\n      |")).toEqual(["back"]);
  });

  it("offers named volumes for a volume entry", () => expect(suggest("    volumes:\n      - |")).toEqual(["data:", "cache:"]));

  it("offers nothing elsewhere", () => {
    expect(suggest("    |")).toEqual([]);
    expect(suggest("    restart: |")).toEqual([]);
    expect(suggest("    healthcheck:\n      |")).toEqual([]);
  });
});
