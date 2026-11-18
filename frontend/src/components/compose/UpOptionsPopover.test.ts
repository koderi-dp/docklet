import { describe, expect, it } from "vitest";
import { parseProfiles } from "./UpOptionsPopover";

describe("parseProfiles", () => {
  it("splits on commas and whitespace", () => {
    expect(parseProfiles("debug, worker  web-2")).toEqual(["debug", "worker", "web-2"]);
    expect(parseProfiles("")).toEqual([]);
  });
  it("rejects names that could be mistaken for flags or break argv", () => {
    expect(parseProfiles("--rm")).toBeNull();
    expect(parseProfiles("a;b")).toBeNull();
  });
});
