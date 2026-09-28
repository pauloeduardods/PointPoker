import { describe, expect, it } from "vitest";
import { computeResults, DECK, formatAverage, isNumericVote } from "../utils/stats";

describe("stats utils", () => {
  it("deck includes coffee and question mark", () => {
    expect(DECK).toContain("☕");
    expect(DECK).toContain("?");
  });

  it("detects numeric votes", () => {
    expect(isNumericVote("0")).toBe(true);
    expect(isNumericVote("13")).toBe(true);
    expect(isNumericVote("?")).toBe(false);
    expect(isNumericVote("☕")).toBe(false);
    expect(isNumericVote("")).toBe(false);
  });

  it("averages numeric votes only, excluding ? and ☕", () => {
    const r = computeResults(["3", "5", "?", "☕", "8"]);
    expect(r.average).toBeCloseTo(16 / 3);
    expect(r.total).toBe(5);
    expect(r.consensus).toBe(false);
  });

  it("detects consensus when all numeric votes are equal (non-numeric ignored)", () => {
    expect(computeResults(["5", "5", "?"]).consensus).toBe(true);
    expect(computeResults(["5", "5", "5"]).consensus).toBe(true);
    expect(computeResults(["5", "8"]).consensus).toBe(false);
  });

  it("has no average or consensus without numeric votes", () => {
    const r = computeResults(["?", "☕"]);
    expect(r.average).toBeNull();
    expect(r.consensus).toBe(false);
    expect(computeResults([]).average).toBeNull();
  });

  it("builds distribution in deck order with percentages, ignoring hidden values", () => {
    const r = computeResults(["8", "3", "8", "?", ""]);
    expect(r.total).toBe(4);
    expect(r.distribution).toEqual([
      { value: "3", count: 1, percent: 25 },
      { value: "8", count: 2, percent: 50 },
      { value: "?", count: 1, percent: 25 },
    ]);
  });

  it("formats averages", () => {
    expect(formatAverage(null)).toBe("—");
    expect(formatAverage(5)).toBe("5");
    expect(formatAverage(16 / 3)).toBe("5.3");
  });
});
