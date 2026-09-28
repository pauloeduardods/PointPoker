import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { apiError, guest, host, makeRound, makeVote, networkError, room } from "./helpers/fixtures";

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

import { useAppStore } from "../stores/app";
import { loadSession, saveSession } from "../utils/session";

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe("app store", () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it("startSession persists token and room code", () => {
    const store = useAppStore();
    store.startSession({ room, participant: host, session_token: "tok" });
    expect(loadSession()).toEqual({ token: "tok", roomCode: "ABC123" });
    expect(store.currentParticipant?.id).toBe(host.id);
    expect(store.isHost).toBe(true);
  });

  it("restores the participant via /me after reload", async () => {
    saveSession({ token: "tok", roomCode: "ABC123" });
    api.roomAPI.getMe.mockResolvedValue(host);
    const store = useAppStore();
    await expect(store.restoreSession("abc123")).resolves.toBe("ok");
    expect(store.isHost).toBe(true);
  });

  it.each([401, 403, 404])("clears the session when /me fails with %i", async (status) => {
    saveSession({ token: "tok", roomCode: "ABC123" });
    api.roomAPI.getMe.mockRejectedValue(apiError(status, "nope"));
    const store = useAppStore();
    await expect(store.restoreSession("ABC123")).resolves.toBe("invalid");
    expect(loadSession()).toBeNull();
  });

  it("rethrows network errors during restore without clearing", async () => {
    saveSession({ token: "tok", roomCode: "ABC123" });
    api.roomAPI.getMe.mockRejectedValue(networkError());
    const store = useAppStore();
    await expect(store.restoreSession("ABC123")).rejects.toBeTruthy();
    expect(loadSession()).not.toBeNull();
  });

  it("returns invalid without calling the API when there is no session for the room", async () => {
    saveSession({ token: "tok", roomCode: "OTHER" });
    const store = useAppStore();
    await expect(store.restoreSession("ABC123")).resolves.toBe("invalid");
    expect(api.roomAPI.getMe).not.toHaveBeenCalled();
  });

  it("computes vote stats and own vote from round state", async () => {
    const store = useAppStore();
    store.startSession({ room, participant: guest, session_token: "tok" });
    api.roomAPI.getRoom.mockResolvedValue({ room, participants: [host, guest] });
    api.votingAPI.getCurrentRound.mockResolvedValue({
      round: makeRound(),
      votes: [makeVote(host.id, ""), makeVote(guest.id, "5")],
    });
    await store.refreshAll();
    expect(store.myVote).toBe("5");
    expect(store.hasVoted).toBe(true);
    expect(store.votedCount).toBe(2);
    expect(store.isOnline(host.id)).toBe(true);
    expect(store.isOnline(guest.id)).toBe(false);
    expect(store.results).toBeNull();

    api.votingAPI.getCurrentRound.mockResolvedValue({
      round: makeRound({ status: "revealed" }),
      votes: [makeVote(host.id, "5"), makeVote(guest.id, "5")],
    });
    await store.refreshRound();
    expect(store.results?.consensus).toBe(true);
    expect(store.results?.average).toBe(5);
  });

  it("host status follows the participant list (host handover)", async () => {
    const store = useAppStore();
    store.startSession({ room, participant: guest, session_token: "tok" });
    expect(store.isHost).toBe(false);
    api.roomAPI.getRoom.mockResolvedValue({ room, participants: [{ ...guest, is_host: true }] });
    await store.refreshRoom();
    expect(store.isHost).toBe(true);
  });

  it("presence replaces the online set", () => {
    const store = useAppStore();
    store.setOnline(["x", "y"]);
    expect(store.isOnline("x")).toBe(true);
    store.setOnline(["y"]);
    expect(store.isOnline("x")).toBe(false);
  });

  it("casts votes optimistically and reverts on failure", async () => {
    const store = useAppStore();
    store.startSession({ room, participant: guest, session_token: "tok" });
    api.roomAPI.getRoom.mockResolvedValue({ room, participants: [host, guest] });
    api.votingAPI.getCurrentRound.mockResolvedValue({ round: makeRound(), votes: [] });
    await store.refreshAll();

    const d = deferred<ReturnType<typeof makeVote>>();
    api.votingAPI.castVote.mockReturnValue(d.promise);
    const p = store.castVote("8");
    expect(store.myVote).toBe("8");
    expect(store.votedCount).toBe(1);
    d.resolve(makeVote(guest.id, "8"));
    await p;
    expect(store.myVote).toBe("8");
    expect(store.pendingVote).toBeNull();

    api.votingAPI.castVote.mockRejectedValue(apiError(409, "round is not voting"));
    await expect(store.castVote("13")).rejects.toBeTruthy();
    expect(store.myVote).toBe("8");
  });

  it("drops stale round fetches that started before a local vote", async () => {
    const store = useAppStore();
    store.startSession({ room, participant: guest, session_token: "tok" });
    api.votingAPI.getCurrentRound.mockResolvedValueOnce({ round: makeRound(), votes: [] });
    await store.refreshRound();

    const stale = deferred<{ round: ReturnType<typeof makeRound>; votes: never[] }>();
    api.votingAPI.getCurrentRound.mockReturnValueOnce(stale.promise);
    const fetching = store.refreshRound();
    api.votingAPI.castVote.mockResolvedValue(makeVote(guest.id, "3"));
    await store.castVote("3");
    stale.resolve({ round: makeRound(), votes: [] });
    await fetching;
    expect(store.myVote).toBe("3");
  });

  it("startRound sets a fresh round; revote clears votes", async () => {
    const store = useAppStore();
    store.startSession({ room, participant: host, session_token: "tok" });
    api.votingAPI.startRound.mockResolvedValue(makeRound({ id: "round-2", story_title: "Next" }));
    await store.startRound("Next");
    expect(api.votingAPI.startRound).toHaveBeenCalledWith("ABC123", "Next");
    expect(store.round?.id).toBe("round-2");

    store.votes = [makeVote(host.id, "5", "round-2")];
    api.votingAPI.reset.mockResolvedValue(makeRound({ id: "round-2", story_title: "Next" }));
    await store.revote();
    expect(api.votingAPI.reset).toHaveBeenCalledWith("ABC123", "round-2");
    expect(store.votes).toEqual([]);
    expect(store.round?.status).toBe("voting");
  });

  it("leaveRoom clears the session even when the API call fails", async () => {
    const store = useAppStore();
    store.startSession({ room, participant: host, session_token: "tok" });
    api.roomAPI.leaveRoom.mockRejectedValue(networkError());
    await store.leaveRoom();
    expect(api.roomAPI.leaveRoom).toHaveBeenCalledWith("ABC123");
    expect(loadSession()).toBeNull();
    expect(store.room).toBeNull();
  });
});
