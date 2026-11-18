import { describe, expect, it } from "vitest";
import { isEligible, needsForceRemove, partitionByEligibility, resolveSelection } from "./actionEligibility";

const t = (id: string, state: string) => ({ id, name: id, state });

describe("action eligibility", () => {
  it("per-state rules", () => {
    expect(isEligible("start", "running")).toBe(false);
    expect(isEligible("start", "exited")).toBe(true);
    expect(isEligible("stop", "exited")).toBe(false);
    expect(isEligible("stop", "running")).toBe(true);
    expect(isEligible("restart", "dead")).toBe(false);
    expect(isEligible("remove", "removing")).toBe(false);
    expect(isEligible("pause", "running")).toBe(true);
    expect(isEligible("pause", "paused")).toBe(false);
    expect(isEligible("unpause", "paused")).toBe(true);
    expect(isEligible("unpause", "running")).toBe(false);
    expect(isEligible("kill", "exited")).toBe(false);
  });

  it("mixed selection yields eligible ids and skipped count", () => {
    const p = partitionByEligibility("stop", [t("a", "running"), t("b", "exited"), t("c", "running"), t("d", "created")]);
    expect(p.eligibleIds).toEqual(["a", "c"]);
    expect(p.skippedCount).toBe(2);
    expect(p.skipped.map((s) => s.id)).toEqual(["b", "d"]);
  });

  it("selection resolves only explicit ids (hidden containers never included implicitly)", () => {
    const all = [t("a", "running"), t("b", "running"), t("c", "exited")];
    expect(resolveSelection(all, new Set(["b"])).map((x) => x.id)).toEqual(["b"]);
    expect(resolveSelection(all, new Set()).length).toBe(0);
  });

  it("force remove only when something is live", () => {
    expect(needsForceRemove([t("a", "exited")])).toBe(false);
    expect(needsForceRemove([t("a", "exited"), t("b", "running")])).toBe(true);
  });
});
