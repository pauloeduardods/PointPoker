import { beforeEach, describe, expect, it } from "vitest";
import { createMemoryHistory } from "vue-router";
import { createAppRouter } from "../router";
import { saveSession } from "../utils/session";

describe("router guard", () => {
  beforeEach(() => localStorage.clear());

  it("redirects to the prefilled join form when there is no session", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/room/abc123");
    expect(router.currentRoute.value.name).toBe("home");
    expect(router.currentRoute.value.query.join).toBe("ABC123");
  });

  it("redirects when the session belongs to another room", async () => {
    saveSession({ token: "t", roomCode: "OTHER1" });
    const router = createAppRouter(createMemoryHistory());
    await router.push("/room/ABC123");
    expect(router.currentRoute.value.query.join).toBe("ABC123");
  });

  it("allows the room when a session for it exists (case-insensitive)", async () => {
    saveSession({ token: "t", roomCode: "ABC123" });
    const router = createAppRouter(createMemoryHistory());
    await router.push("/room/abc123");
    expect(router.currentRoute.value.name).toBe("room");
  });

  it("redirects unknown paths home", async () => {
    const router = createAppRouter(createMemoryHistory());
    await router.push("/nope/here");
    expect(router.currentRoute.value.name).toBe("home");
  });
});
