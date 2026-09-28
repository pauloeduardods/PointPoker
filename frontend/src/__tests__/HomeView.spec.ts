import { beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createMemoryHistory, type Router } from "vue-router";
import { apiError, guest, host, room } from "./helpers/fixtures";

const api = vi.hoisted(() => ({
  roomAPI: {
    createRoom: vi.fn(),
    getRoom: vi.fn(),
    joinRoom: vi.fn(),
    getMe: vi.fn(),
    leaveRoom: vi.fn(),
  },
}));
vi.mock("../api/rooms", () => ({ roomAPI: api.roomAPI }));

import HomeView from "../views/HomeView.vue";
import { createAppRouter } from "../router";
import { loadSession } from "../utils/session";

async function mountAt(path: string) {
  const pinia = createPinia();
  setActivePinia(pinia);
  const router: Router = createAppRouter(createMemoryHistory());
  // Avoid loading RoomView (and its side effects) during navigation.
  router.beforeEach((to) => (to.name === "room" ? false : true));
  const pushSpy = vi.spyOn(router, "push");
  await router.push(path);
  await router.isReady();
  pushSpy.mockClear();
  const wrapper = mount(HomeView, { global: { plugins: [pinia, router] }, attachTo: document.body });
  return { wrapper, router, pushSpy };
}

describe("HomeView", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
    document.body.innerHTML = "";
  });

  it("creates a room, stores the session and navigates", async () => {
    api.roomAPI.createRoom.mockResolvedValue({ room, participant: host, session_token: "tok" });
    const { wrapper, pushSpy } = await mountAt("/");
    await wrapper.get("#create-room-name").setValue("  Sprint planning ");
    await wrapper.get("#create-display-name").setValue("Alice");
    await wrapper.get("[data-testid=create-form]").trigger("submit");
    await flushPromises();

    expect(api.roomAPI.createRoom).toHaveBeenCalledWith("Sprint planning", "Alice");
    expect(loadSession()).toEqual({ token: "tok", roomCode: "ABC123" });
    expect(pushSpy).toHaveBeenCalledWith({ name: "room", params: { code: "ABC123" } });
  });

  it("joins a room with an upper-cased code", async () => {
    api.roomAPI.joinRoom.mockResolvedValue({ room, participant: guest, session_token: "tok2" });
    const { wrapper, pushSpy } = await mountAt("/");
    await wrapper.get("[data-testid=tab-join]").trigger("click");
    await wrapper.get("#join-room-code").setValue("abc123");
    await wrapper.get("#join-display-name").setValue("Bob");
    await wrapper.get("[data-testid=join-form]").trigger("submit");
    await flushPromises();

    expect(api.roomAPI.joinRoom).toHaveBeenCalledWith("ABC123", "Bob");
    expect(loadSession()?.token).toBe("tok2");
    expect(pushSpy).toHaveBeenCalledWith({ name: "room", params: { code: "ABC123" } });
  });

  it("prefills the join tab from ?join=CODE", async () => {
    const { wrapper } = await mountAt("/?join=xyz789");
    expect(wrapper.find("[data-testid=join-form]").exists()).toBe(true);
    expect((wrapper.get("#join-room-code").element as HTMLInputElement).value).toBe("XYZ789");
    expect(wrapper.get("[data-testid=invite-hint]").text()).toContain("XYZ789");
    expect(wrapper.get("[data-testid=tab-join]").attributes("aria-selected")).toBe("true");
  });

  it("shows the API error message on failure", async () => {
    api.roomAPI.joinRoom.mockRejectedValue(apiError(404, "room not found"));
    const { wrapper, pushSpy } = await mountAt("/?join=NOPE00");
    await wrapper.get("#join-display-name").setValue("Bob");
    await wrapper.get("[data-testid=join-form]").trigger("submit");
    await flushPromises();
    expect(wrapper.get("[role=alert]").text()).toBe("Room not found");
    expect(pushSpy).not.toHaveBeenCalled();
    expect(loadSession()).toBeNull();
  });

  it("labels are tied to inputs", async () => {
    const { wrapper } = await mountAt("/");
    for (const label of wrapper.findAll("label")) {
      const id = label.attributes("for");
      expect(id).toBeTruthy();
      expect(wrapper.find(`#${id}`).exists()).toBe(true);
    }
  });
});
