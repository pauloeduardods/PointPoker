import { defineStore } from "pinia";
import { ref, computed } from "vue";
import type { Room, Participant } from "../api/rooms";
import type { VotingRound, Vote } from "../api/voting";

export const useAppStore = defineStore("app", () => {
  // State
  const currentRoom = ref<Room | null>(null);
  const currentParticipant = ref<Participant | null>(null);
  const participants = ref<Participant[]>([]);
  const currentRound = ref<VotingRound | null>(null);
  const votes = ref<Vote[]>([]);
  const sessionToken = ref<string | null>(localStorage.getItem("sessionToken"));
  const wsConnected = ref(false);
  const loading = ref(false);
  const error = ref<string | null>(null);

  // Computed
  const isHost = computed(() => currentParticipant.value?.is_host ?? false);
  const roomCode = computed(() => currentRoom.value?.code ?? "");
  const hasVoted = computed(
    () =>
      votes.value.some(
        (v) => v.participant_id === currentParticipant.value?.id,
      ) && currentRound.value?.status === "voting",
  );

  const votesByParticipant = computed(() => {
    const result: Record<string, Vote | null> = {};
    participants.value.forEach((p) => {
      result[p.id] = votes.value.find((v) => v.participant_id === p.id) || null;
    });
    return result;
  });

  const roundStats = computed(() => {
    const stats = {
      totalVotes: 0,
      totalParticipants: participants.value.length,
      voteValues: {} as Record<string, number>,
    };

    votes.value.forEach((vote) => {
      stats.totalVotes++;
      stats.voteValues[vote.value] = (stats.voteValues[vote.value] || 0) + 1;
    });

    return stats;
  });

  // Actions
  const setRoom = (room: Room | null) => {
    currentRoom.value = room;
  };

  const setParticipant = (participant: Participant | null) => {
    currentParticipant.value = participant;
  };

  const setParticipants = (newParticipants: Participant[]) => {
    participants.value = newParticipants;
  };

  const addParticipant = (participant: Participant) => {
    if (!participants.value.find((p) => p.id === participant.id)) {
      participants.value.push(participant);
    }
  };

  const removeParticipant = (participantId: string) => {
    participants.value = participants.value.filter(
      (p) => p.id !== participantId,
    );
  };

  const setCurrentRound = (round: VotingRound | null) => {
    currentRound.value = round;
  };

  const setVotes = (newVotes: Vote[]) => {
    votes.value = newVotes;
  };

  const addOrUpdateVote = (vote: Vote) => {
    const existing = votes.value.findIndex(
      (v) => v.participant_id === vote.participant_id,
    );
    if (existing >= 0) {
      votes.value[existing] = vote;
    } else {
      votes.value.push(vote);
    }
  };

  const setSessionToken = (token: string) => {
    sessionToken.value = token;
    localStorage.setItem("sessionToken", token);
  };

  const clearSession = () => {
    sessionToken.value = null;
    currentRoom.value = null;
    currentParticipant.value = null;
    participants.value = [];
    currentRound.value = null;
    votes.value = [];
    localStorage.removeItem("sessionToken");
  };

  const setWsConnected = (connected: boolean) => {
    wsConnected.value = connected;
  };

  const setLoading = (isLoading: boolean) => {
    loading.value = isLoading;
  };

  const setError = (message: string | null) => {
    error.value = message;
  };

  const resetRound = () => {
    currentRound.value = null;
    votes.value = [];
  };

  return {
    // State
    currentRoom,
    currentParticipant,
    participants,
    currentRound,
    votes,
    sessionToken,
    wsConnected,
    loading,
    error,

    // Computed
    isHost,
    roomCode,
    hasVoted,
    votesByParticipant,
    roundStats,

    // Actions
    setRoom,
    setParticipant,
    setParticipants,
    addParticipant,
    removeParticipant,
    setCurrentRound,
    setVotes,
    addOrUpdateVote,
    setSessionToken,
    clearSession,
    setWsConnected,
    setLoading,
    setError,
    resetRound,
  };
});
