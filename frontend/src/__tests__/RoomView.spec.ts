import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createMemoryHistory, type Router } from "vue-router";
import type { Participant, Round, Vote } from "../api/types";
import { FakeWebSocket } from "./helpers/fakeWebSocket";
import { apiError, guest, host, makeRound, makeVote, room } from "./helpers/fixtures";

const api = vi.hoisted(() => ({
  roomAPI: {
    createRoom: vi.fn(),
    getRoom: vi.fn(),
    joinRoom: vi.fn(),
    getMe: vi.fn(),
    leaveRoom: vi.fn(),
  },
  votingAPI: {
    startRound: vi.fn(),
    getCurrentRound: vi.fn(),
    castVote: vi.fn(),
    reveal: vi.fn(),
    reset: vi.fn(),
  },
}));
vi.mock("../api/rooms", () => ({ roomAPI: api.roomAPI }));
vi.mock("../api/voting", () => ({ votingAPI: api.votingAPI }));

import RoomView from "../views/RoomView.vue";
import { createAppRouter } from "../router";
import { loadSession, saveSession } from "../utils/session";

interface Scenario {
  me: Participant;
  participants?: Participant[];
  round?: Round | null;
  votes?: Vote[];
}

function serve({ me, participants = [host, guest], round = null, votes = [] }: Scenario) {
  api.roomAPI.getMe.mockResolvedValue(me);
  api.roomAPI.getRoom.mockResolvedValue({ room, participants });
  api.votingAPI.getCurrentRound.mockResolvedValue({ round, votes });
}

async function mountRoom() {
  const pinia = createPinia();
  setActivePinia(pinia);
  const router: Router = createAppRouter(createMemoryHistory());
  await router.push("/room/ABC123");
  await router.isReady();
  const wrapper = mount(RoomView, { global: { plugins: [pinia, router] } });
  await flushPromises();
  return { wrapper, router };
}

const pressed = (w: ReturnType<typeof mount>, value: string) =>
  w.get(`[data-testid="card-${value}"]`).attributes("aria-pressed");

describe("RoomView", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
    FakeWebSocket.reset();
    vi.stubGlobal("WebSocket", FakeWebSocket);
    saveSession({ token: "tok", roomCode: "ABC123" });
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("restores the session via /me and shows host controls for the host", async () => {
    serve({ me: host });
    const { wrapper } = await mountRoom();
    expect(api.roomAPI.getMe).toHaveBeenCalledWith("ABC123");
    expect(wrapper.get("[data-testid=room-name]").text()).toBe("Sprint planning");
    expect(wrapper.get("[data-testid=room-code]").text()).toBe("ABC123");
    expect(wrapper.find("[data-testid=host-controls]").exists()).toBe(true);
    expect(wrapper.find("[data-testid=next-story-form]").exists()).toBe(true);
    expect(FakeWebSocket.latest().url).toBe(`ws://${location.host}/api/rooms/ABC123/ws?token=tok`);
  });

  it("guests see no host controls and a waiting message", async () => {
    serve({ me: guest });
    const { wrapper } = await mountRoom();
    expect(wrapper.find("[data-testid=host-controls]").exists()).toBe(false);
    expect(wrapper.find("[data-testid=waiting-host]").exists()).toBe(true);
    expect(wrapper.get(`[data-testid=participant-${guest.id}]`).text()).toContain("(you)");
    expect(wrapper.get(`[data-testid=participant-${host.id}]`).text()).toContain("Host");
  });

  it("redirects to the prefilled join form when the token is rejected", async () => {
    api.roomAPI.getMe.mockRejectedValue(apiError(401, "invalid session"));
    const { router } = await mountRoom();
    expect(router.currentRoute.value.name).toBe("home");
    expect(router.currentRoute.value.query.join).toBe("ABC123");
    expect(loadSession()).toBeNull();
    expect(FakeWebSocket.instances).toHaveLength(0);
  });

  it("highlights my own vote and changes it optimistically", async () => {
    serve({ me: guest, round: makeRound(), votes: [makeVote(host.id, ""), makeVote(guest.id, "5")] });
    const { wrapper } = await mountRoom();
    expect(pressed(wrapper, "5")).toBe("true");
    expect(wrapper.get("[data-testid=voted-count]").text()).toBe("2/2 voted");

    api.votingAPI.castVote.mockReturnValue(new Promise(() => {})); // never resolves
    await wrapper.get('[data-testid="card-8"]').trigger("click");
    expect(api.votingAPI.castVote).toHaveBeenCalledWith("ABC123", "round-1", "8");
    expect(pressed(wrapper, "8")).toBe("true");
    expect(pressed(wrapper, "5")).toBe("false");
    // Other cards remain clickable (no global loading lock).
    expect(wrapper.get('[data-testid="card-13"]').attributes("disabled")).toBeUndefined();
  });

  it("reverts the vote and shows a friendly error when voting fails", async () => {
    serve({ me: guest, round: makeRound(), votes: [] });
    const { wrapper } = await mountRoom();
    api.votingAPI.castVote.mockRejectedValue(apiError(409, "round is not accepting votes"));
    await wrapper.get('[data-testid="card-3"]').trigger("click");
    await flushPromises();
    expect(pressed(wrapper, "3")).toBe("false");
    expect(wrapper.get("[data-testid=action-error]").text()).toContain("Round is not accepting votes");
  });

  it("host reveals and sees results with average and consensus", async () => {
    serve({ me: host, round: makeRound(), votes: [makeVote(host.id, "5"), makeVote(guest.id, "")] });
    const { wrapper } = await mountRoom();
    api.votingAPI.reveal.mockResolvedValue([]);
    api.votingAPI.getCurrentRound.mockResolvedValue({
      round: makeRound({ status: "revealed" }),
      votes: [makeVote(host.id, "5"), makeVote(guest.id, "5")],
    });
    await wrapper.get("[data-testid=reveal]").trigger("click");
    await flushPromises();

    expect(api.votingAPI.reveal).toHaveBeenCalledWith("ABC123", "round-1");
    expect(wrapper.get("[data-testid=round-status]").text()).toBe("Revealed");
    expect(wrapper.get("[data-testid=average]").text()).toBe("5");
    expect(wrapper.find("[data-testid=consensus]").exists()).toBe(true);
    const individual = wrapper.get("[data-testid=individual-votes]").text();
    expect(individual).toContain("Alice");
    expect(individual).toContain("Bob");
  });

  it("shows mixed results without consensus, excluding ? and ☕ from the average", async () => {
    serve({
      me: guest,
      participants: [host, guest, { ...guest, id: "p3", display_name: "Cy" }],
      round: makeRound({ status: "revealed" }),
      votes: [makeVote(host.id, "3"), makeVote(guest.id, "8"), makeVote("p3", "☕")],
    });
    const { wrapper } = await mountRoom();
    expect(wrapper.get("[data-testid=average]").text()).toBe("5.5");
    expect(wrapper.find("[data-testid=consensus]").exists()).toBe(false);
    expect(wrapper.get("[data-testid=distribution]").findAll("li")).toHaveLength(3);
  });

  it("after reveal the host can revote or start the next story", async () => {
    serve({ me: host, round: makeRound({ status: "revealed" }), votes: [makeVote(host.id, "5")] });
    const { wrapper } = await mountRoom();

    api.votingAPI.reset.mockResolvedValue(makeRound());
    await wrapper.get("[data-testid=revote]").trigger("click");
    await flushPromises();
    expect(api.votingAPI.reset).toHaveBeenCalledWith("ABC123", "round-1");
    expect(wrapper.get("[data-testid=round-status]").text()).toBe("Voting");
    expect(pressed(wrapper, "5")).toBe("false");

    // While voting, "Next story" reveals the form.
    await wrapper.get("[data-testid=skip-story]").trigger("click");
    api.votingAPI.startRound.mockResolvedValue(makeRound({ id: "round-2", story_title: "Checkout" }));
    await wrapper.get("#story-title").setValue("Checkout");
    await wrapper.get("[data-testid=next-story-form]").trigger("submit");
    await flushPromises();
    expect(api.votingAPI.startRound).toHaveBeenCalledWith("ABC123", "Checkout");
    expect(wrapper.get("[data-testid=story-title]").text()).toBe("Checkout");
  });

  it("presence events update online dots; other events trigger a debounced refetch", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "setInterval", "clearInterval"] });
    serve({ me: host });
    const { wrapper } = await mountRoom();
    const ws = FakeWebSocket.latest();
    ws.serverOpen();
    await flushPromises();
    expect(wrapper.get("[data-testid=connection-status]").text()).toBe("Live");

    const dot = () =>
      wrapper.get(`[data-testid=participant-${guest.id}] [data-testid=online-dot]`).attributes("data-online");
    expect(dot()).toBe("false");
    ws.emit("presence", { online: [host.id, guest.id] });
    await flushPromises();
    expect(dot()).toBe("true");

    const calls = api.roomAPI.getRoom.mock.calls.length;
    ws.emit("vote_cast", { participant_id: guest.id });
    ws.emit("vote_cast", { participant_id: host.id });
    ws.emit("participant_joined", { participant: guest });
    vi.advanceTimersByTime(149);
    expect(api.roomAPI.getRoom).toHaveBeenCalledTimes(calls);
    vi.advanceTimersByTime(1);
    expect(api.roomAPI.getRoom).toHaveBeenCalledTimes(calls + 1);
    expect(api.votingAPI.getCurrentRound).toHaveBeenCalledTimes(calls + 1);
  });

  it("polls every 5s only while the socket is disconnected", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "setInterval", "clearInterval"] });
    serve({ me: guest });
    const { wrapper } = await mountRoom();
    FakeWebSocket.latest().serverOpen();
    const base = api.roomAPI.getRoom.mock.calls.length;
    vi.advanceTimersByTime(20_000);
    expect(api.roomAPI.getRoom).toHaveBeenCalledTimes(base);

    FakeWebSocket.latest().serverClose();
    await flushPromises();
    expect(wrapper.get("[data-testid=connection-status]").text()).toBe("Reconnecting…");
    // Make reconnect attempts fail so we stay disconnected for a while.
    vi.advanceTimersByTime(500);
    FakeWebSocket.latest().serverClose();
    vi.advanceTimersByTime(4500);
    expect(api.roomAPI.getRoom).toHaveBeenCalledTimes(base + 1);

    FakeWebSocket.latest().serverClose();
    const sockets = FakeWebSocket.instances.length;
    vi.advanceTimersByTime(2000); // backoff: 500, 1000, 2000…
    expect(FakeWebSocket.instances).toHaveLength(sockets + 1);
    FakeWebSocket.latest().serverOpen(); // back online → one catch-up refetch, polling stops
    vi.advanceTimersByTime(150);
    const afterReconnect = api.roomAPI.getRoom.mock.calls.length;
    vi.advanceTimersByTime(20_000);
    expect(api.roomAPI.getRoom).toHaveBeenCalledTimes(afterReconnect);
    wrapper.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("leave calls the API, clears the session and goes home", async () => {
    serve({ me: guest });
    const { wrapper, router } = await mountRoom();
    vi.spyOn(window, "confirm").mockReturnValue(true);
    api.roomAPI.leaveRoom.mockRejectedValue(apiError(500));
    await wrapper.get("[data-testid=leave]").trigger("click");
    await flushPromises();
    expect(api.roomAPI.leaveRoom).toHaveBeenCalledWith("ABC123");
    expect(loadSession()).toBeNull();
    expect(router.currentRoute.value.name).toBe("home");
    expect(FakeWebSocket.latest().closeCalls).toBe(1);
  });

  it("does nothing when leaving is cancelled", async () => {
    serve({ me: guest });
    const { wrapper } = await mountRoom();
    vi.spyOn(window, "confirm").mockReturnValue(false);
    await wrapper.get("[data-testid=leave]").trigger("click");
    expect(api.roomAPI.leaveRoom).not.toHaveBeenCalled();
    expect(loadSession()).not.toBeNull();
  });

  it("copies the invite link", async () => {
    serve({ me: guest });
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText } });
    vi.stubGlobal("isSecureContext", true);
    const { wrapper } = await mountRoom();
    await wrapper.get("[data-testid=copy-link]").trigger("click");
    await flushPromises();
    expect(writeText).toHaveBeenCalledWith(`${location.origin}/room/ABC123`);
    expect(wrapper.get("[data-testid=copy-link]").text()).toBe("Link copied!");
  });
});
