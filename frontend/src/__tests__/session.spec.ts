import { beforeEach, describe, expect, it } from "vitest";
import {
  clearStoredSession,
  hasSessionFor,
  loadSession,
  saveSession,
  SESSION_STORAGE_KEY,
} from "../utils/session";

describe("session persistence", () => {
  beforeEach(() => localStorage.clear());

  it("round-trips token and room code (normalized)", () => {
    saveSession({ token: "tok", roomCode: "abc123" });
    expect(loadSession()).toEqual({ token: "tok", roomCode: "ABC123" });
    expect(hasSessionFor("abc123")).toBe(true);
    expect(hasSessionFor("OTHER")).toBe(false);
  });

  it("returns null for missing or malformed data", () => {
    expect(loadSession()).toBeNull();
    localStorage.setItem(SESSION_STORAGE_KEY, "{not json");
    expect(loadSession()).toBeNull();
    localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify({ token: "x" }));
    expect(loadSession()).toBeNull();
  });

  it("clears stored session including the legacy key", () => {
    saveSession({ token: "tok", roomCode: "ABC" });
    localStorage.setItem("sessionToken", "old");
    clearStoredSession();
    expect(loadSession()).toBeNull();
    expect(localStorage.getItem("sessionToken")).toBeNull();
  });
});
