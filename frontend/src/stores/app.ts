import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { roomAPI } from "../api/rooms";
import { votingAPI } from "../api/voting";
import { errorStatus } from "../api/client";
import type {
  Participant,
  Room,
  Round,
  SessionResponse,
  Vote,
} from "../api/types";
import {
  clearStoredSession,
  loadSession,
  normalizeRoomCode,
  saveSession,
  type StoredSession,
} from "../utils/session";
import { computeResults, type RoundResults } from "../utils/stats";

export type RestoreResult = "ok" | "invalid";

export const useAppStore = defineStore("app", () => {
  // ---- state ----
  const session = ref<StoredSession | null>(loadSession());
  const room = ref<Room | null>(null);
  const participants = ref<Participant[]>([]);
  const myId = ref<string | null>(null);
  /** Participant as returned by /me or create/join; used until the room list loads. */
  const meSnapshot = ref<Participant | null>(null);
  const round = ref<Round | null>(null);
  const votes = ref<Vote[]>([]);
  const onlineIds = ref<Set<string>>(new Set());
  const wsConnected = ref(false);
  /** Optimistic vote value awaiting server confirmation. */
  const pendingVote = ref<string | null>(null);

  /** Bumped on every local round mutation so stale fetch responses are dropped. */
  let roundSeq = 0;

  // ---- getters ----
  const sessionToken = computed(() => session.value?.token ?? null);
  const roomCode = computed(
    () => room.value?.code ?? session.value?.roomCode ?? "",
  );

  const currentParticipant = computed<Participant | null>(() => {
    if (!myId.value) return null;
    return (
      participants.value.find((p) => p.id === myId.value) ?? meSnapshot.value
    );
  });

  const isHost = computed(() => currentParticipant.value?.is_host ?? false);

  /** Participants who have voted in the current round (including my pending vote). */
  const votedIds = computed(() => {
    const ids = new Set(votes.value.map((v) => v.participant_id));
    if (pendingVote.value !== null && myId.value) ids.add(myId.value);
    return ids;
  });

  const myVote = computed<string | null>(() => {
    if (pendingVote.value !== null) return pendingVote.value;
    if (!myId.value) return null;
    const v = votes.value.find((x) => x.participant_id === myId.value);
    return v && v.value !== "" ? v.value : null;
  });

  const hasVoted = computed(
    () => myId.value !== null && votedIds.value.has(myId.value),
  );

  const votedCount = computed(
    () => participants.value.filter((p) => votedIds.value.has(p.id)).length,
  );

  const results = computed<RoundResults | null>(() => {
    if (round.value?.status !== "revealed") return null;
    return computeResults(votes.value.map((v) => v.value));
  });

  function isOnline(id: string): boolean {
    return onlineIds.value.has(id);
  }

  function voteOf(participantId: string): string | null {
    const v = votes.value.find((x) => x.participant_id === participantId);
    return v ? v.value : null;
  }

  // ---- session ----
  function startSession(res: SessionResponse): void {
    const code = normalizeRoomCode(res.room.code);
    session.value = { token: res.session_token, roomCode: code };
    saveSession(session.value);
    room.value = res.room;
    myId.value = res.participant.id;
    meSnapshot.value = res.participant;
    participants.value = [];
    round.value = null;
    votes.value = [];
    onlineIds.value = new Set();
  }

  function clearSession(): void {
    clearStoredSession();
    session.value = null;
    room.value = null;
    participants.value = [];
    myId.value = null;
    meSnapshot.value = null;
    round.value = null;
    votes.value = [];
    onlineIds.value = new Set();
    wsConnected.value = false;
    pendingVote.value = null;
    roundSeq++;
  }

  /**
   * Validate the stored session against the given room.
   * Returns "invalid" (and clears the session) when there is no usable token.
   * Network/server errors are re-thrown.
   */
  async function restoreSession(code: string): Promise<RestoreResult> {
    const normalized = normalizeRoomCode(code);
    session.value = loadSession();
    if (!session.value || session.value.roomCode !== normalized) {
      return "invalid";
    }
    try {
      const me = await roomAPI.getMe(normalized);
      myId.value = me.id;
      meSnapshot.value = me;
      return "ok";
    } catch (err) {
      const status = errorStatus(err);
      if (status === 401 || status === 403 || status === 404) {
        clearSession();
        return "invalid";
      }
      throw err;
    }
  }

  // ---- data loading ----
  async function refreshRoom(): Promise<void> {
    const code = roomCode.value;
    if (!code) return;
    const data = await roomAPI.getRoom(code);
    room.value = data.room;
    participants.value = data.participants;
    if (data.participants.some((p) => typeof p.online === "boolean")) {
      onlineIds.value = new Set(
        data.participants.filter((p) => p.online).map((p) => p.id),
      );
    }
    const me = data.participants.find((p) => p.id === myId.value);
    if (me) meSnapshot.value = me;
  }

  async function refreshRound(): Promise<void> {
    const code = roomCode.value;
    if (!code) return;
    const seq = ++roundSeq;
    const data = await votingAPI.getCurrentRound(code);
    if (seq !== roundSeq) return; // a newer fetch or local change superseded this one
    applyRound(data.round, data.votes);
  }

  async function refreshAll(): Promise<void> {
    await Promise.all([refreshRoom(), refreshRound()]);
  }

  function applyRound(nextRound: Round | null, nextVotes: Vote[]): void {
    if (nextRound?.id !== round.value?.id) pendingVote.value = null;
    round.value = nextRound;
    votes.value = nextVotes;
  }

  function setOnline(ids: readonly string[]): void {
    onlineIds.value = new Set(ids);
  }

  function setWsConnected(connected: boolean): void {
    wsConnected.value = connected;
  }

  // ---- actions ----
  /** Optimistically cast/change a vote. Reverts and re-throws on failure. */
  async function castVote(value: string): Promise<void> {
    const r = round.value;
    const me = myId.value;
    if (!r || r.status !== "voting" || !me) return;
    roundSeq++;
    pendingVote.value = value;
    try {
      const vote = await votingAPI.castVote(roomCode.value, r.id, value);
      if (round.value?.id !== r.id) return;
      const saved: Vote = { ...vote, value: vote.value || value };
      const idx = votes.value.findIndex((v) => v.participant_id === me);
      if (idx >= 0) votes.value.splice(idx, 1, saved);
      else votes.value.push(saved);
      if (pendingVote.value === value) pendingVote.value = null;
    } catch (err) {
      if (pendingVote.value === value) pendingVote.value = null;
      throw err;
    }
  }

  async function startRound(storyTitle: string): Promise<void> {
    const newRound = await votingAPI.startRound(roomCode.value, storyTitle);
    roundSeq++;
    applyRound(newRound, []);
  }

  async function revealVotes(): Promise<void> {
    const r = round.value;
    if (!r) return;
    await votingAPI.reveal(roomCode.value, r.id);
    await refreshRound();
  }

  async function revote(): Promise<void> {
    const r = round.value;
    if (!r) return;
    const updated = await votingAPI.reset(roomCode.value, r.id);
    roundSeq++;
    pendingVote.value = null;
    applyRound(updated, []);
  }

  /** Leave the room on the server (best effort) and always clear locally. */
  async function leaveRoom(): Promise<void> {
    const code = roomCode.value;
    try {
      if (code && session.value) await roomAPI.leaveRoom(code);
    } catch {
      // Clear locally regardless.
    } finally {
      clearSession();
    }
  }

  return {
    // state
    session,
    room,
    participants,
    myId,
    round,
    votes,
    onlineIds,
    wsConnected,
    pendingVote,
    // getters
    sessionToken,
    roomCode,
    currentParticipant,
    isHost,
    votedIds,
    myVote,
    hasVoted,
    votedCount,
    results,
    isOnline,
    voteOf,
    // actions
    startSession,
    clearSession,
    restoreSession,
    refreshRoom,
    refreshRound,
    refreshAll,
    setOnline,
    setWsConnected,
    castVote,
    startRound,
    revealVotes,
    revote,
    leaveRoom,
  };
});
