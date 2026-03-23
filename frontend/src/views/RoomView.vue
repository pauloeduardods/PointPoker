<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow sticky top-0 z-10">
      <div
        class="max-w-7xl mx-auto px-4 py-4 sm:px-6 lg:px-8 flex justify-between items-center"
      >
        <div>
          <h1 class="text-2xl font-bold text-gray-900">Point Poker</h1>
          <p class="text-sm text-gray-600">Room: {{ store.roomCode }}</p>
        </div>
        <button
          @click="handleLeaveRoom"
          class="px-4 py-2 bg-red-500 hover:bg-red-600 text-white rounded-lg font-semibold transition"
        >
          Leave
        </button>
      </div>
    </header>

    <!-- Main Content -->
    <main class="max-w-7xl mx-auto px-4 py-8 sm:px-6 lg:px-8">
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-8">
        <!-- Main Area -->
        <div class="lg:col-span-2">
          <!-- Round Section -->
          <div class="bg-white rounded-lg shadow p-6 mb-8">
            <div v-if="!store.currentRound" class="text-center py-8">
              <h2 class="text-xl font-semibold text-gray-800 mb-4">
                No active round
              </h2>
              <p class="text-gray-600 mb-6">
                Start a new round to begin voting
              </p>

              <div v-if="store.isHost" class="space-y-4 max-w-md mx-auto">
                <input
                  v-model="newRoundTitle"
                  type="text"
                  placeholder="Enter story title (e.g., User login form)"
                  class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                  @keyup.enter="handleStartRound"
                />
                <button
                  @click="handleStartRound"
                  :disabled="!newRoundTitle || loading"
                  class="w-full bg-blue-500 hover:bg-blue-600 disabled:bg-gray-400 text-white font-semibold py-2 px-4 rounded-lg transition"
                >
                  <span v-if="loading">Starting...</span>
                  <span v-else>Start Round</span>
                </button>
              </div>
              <p v-else class="text-gray-500 text-sm">
                Waiting for host to start a round...
              </p>
            </div>

            <div v-else>
              <!-- Round Info -->
              <div class="mb-6">
                <h2 class="text-2xl font-bold text-gray-900">
                  {{ store.currentRound.story_title }}
                </h2>
                <div class="mt-2 flex items-center space-x-4">
                  <span
                    :class="[
                      'px-3 py-1 rounded-full text-sm font-semibold',
                      store.currentRound.status === 'voting'
                        ? 'bg-yellow-100 text-yellow-800'
                        : 'bg-green-100 text-green-800',
                    ]"
                  >
                    {{
                      store.currentRound.status === "voting"
                        ? "🎯 Voting"
                        : "✓ Revealed"
                    }}
                  </span>
                </div>
              </div>

              <!-- Voting Cards -->
              <div v-if="store.currentRound.status === 'voting'" class="mb-6">
                <h3 class="text-lg font-semibold text-gray-800 mb-4">
                  Cast Your Vote
                </h3>
                <div class="grid grid-cols-3 sm:grid-cols-5 gap-3">
                  <button
                    v-for="value in fibonacciDeck"
                    :key="value"
                    @click="handleVote(value)"
                    :class="[
                      'py-4 px-2 rounded-lg font-bold text-lg transition',
                      store.currentParticipant?.id &&
                      store.votesByParticipant[store.currentParticipant.id]
                        ?.value === value
                        ? 'bg-blue-500 text-white ring-4 ring-blue-300'
                        : 'bg-gray-100 text-gray-700 hover:bg-gray-200',
                    ]"
                  >
                    {{ value }}
                  </button>
                </div>
                <p v-if="store.hasVoted" class="text-sm text-green-600 mt-3">
                  ✓ Your vote has been recorded
                </p>
              </div>

              <!-- Vote Results -->
              <div v-if="store.currentRound.status === 'revealed'" class="mb-6">
                <h3 class="text-lg font-semibold text-gray-800 mb-4">
                  Results
                </h3>
                <div class="space-y-3">
                  <div
                    v-for="(count, value) in store.roundStats.voteValues"
                    :key="value"
                    class="flex items-center"
                  >
                    <div class="w-16">
                      <span class="text-lg font-bold text-blue-600">{{
                        value
                      }}</span>
                    </div>
                    <div class="flex-1">
                      <div
                        class="bg-gray-200 rounded-full h-8 flex items-center"
                      >
                        <div
                          :style="{
                            width: `${(count / store.roundStats.totalVotes) * 100}%`,
                          }"
                          class="bg-blue-500 h-8 rounded-full transition-all duration-300 flex items-center justify-center"
                        >
                          <span
                            v-if="
                              (count / store.roundStats.totalVotes) * 100 > 15
                            "
                            class="text-white text-sm font-semibold"
                          >
                            {{ count }}
                          </span>
                        </div>
                      </div>
                    </div>
                    <div class="w-12 text-right">
                      <span class="text-gray-700"
                        >{{ count }}/{{
                          store.roundStats.totalParticipants
                        }}</span
                      >
                    </div>
                  </div>
                </div>

                <!-- Actions -->
                <div v-if="store.isHost" class="mt-6 space-y-2">
                  <button
                    @click="handleResetRound"
                    :disabled="loading"
                    class="w-full bg-green-500 hover:bg-green-600 disabled:bg-gray-400 text-white font-semibold py-2 px-4 rounded-lg transition"
                  >
                    <span v-if="loading">Resetting...</span>
                    <span v-else>Start New Round</span>
                  </button>
                </div>
              </div>

              <!-- Host Controls -->
              <div
                v-if="store.isHost && store.currentRound.status === 'voting'"
                class="mt-6 pt-6 border-t border-gray-200"
              >
                <button
                  @click="handleRevealVotes"
                  :disabled="store.roundStats.totalVotes === 0 || loading"
                  class="w-full bg-purple-500 hover:bg-purple-600 disabled:bg-gray-400 text-white font-semibold py-2 px-4 rounded-lg transition"
                >
                  <span v-if="loading">Revealing...</span>
                  <span v-else
                    >Reveal Votes ({{ store.roundStats.totalVotes }}/{{
                      store.roundStats.totalParticipants
                    }})</span
                  >
                </button>
              </div>
            </div>
          </div>

          <!-- Error Message -->
          <div
            v-if="error"
            class="bg-red-50 border border-red-200 rounded-lg p-4 mb-8"
          >
            <p class="text-red-700">{{ error }}</p>
          </div>
        </div>

        <!-- Sidebar -->
        <aside class="space-y-6">
          <!-- Participants -->
          <div class="bg-white rounded-lg shadow p-6">
            <h3 class="text-lg font-semibold text-gray-800 mb-4">
              Participants ({{ store.participants.length }})
            </h3>
            <div class="space-y-2">
              <div
                v-for="participant in store.participants"
                :key="participant.id"
                :class="[
                  'p-3 rounded-lg flex items-center justify-between',
                  participant.id === store.currentParticipant?.id
                    ? 'bg-blue-50 border border-blue-200'
                    : 'bg-gray-50',
                ]"
              >
                <div>
                  <p class="font-semibold text-gray-900">
                    {{ participant.display_name }}
                  </p>
                  <p class="text-xs text-gray-600">
                    {{ participant.is_host ? "Host" : "Guest" }}
                  </p>
                </div>
                <div v-if="store.currentRound" class="text-right">
                  <div v-if="store.votesByParticipant[participant.id]">
                    <span
                      v-if="store.currentRound.status === 'voting'"
                      class="text-sm font-semibold text-green-600"
                    >
                      ✓ Voted
                    </span>
                    <span v-else class="text-lg font-bold text-blue-600">
                      {{ store?.votesByParticipant[participant.id]?.value }}
                    </span>
                  </div>
                  <span v-else class="text-sm text-gray-500">Waiting...</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Connection Status -->
          <div
            :class="[
              'rounded-lg p-4 text-sm font-semibold',
              store.wsConnected
                ? 'bg-green-50 text-green-700 border border-green-200'
                : 'bg-red-50 text-red-700 border border-red-200',
            ]"
          >
            <span v-if="store.wsConnected">🟢 Connected</span>
            <span v-else>🔴 Disconnected</span>
          </div>
        </aside>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { useRouter, useRoute } from "vue-router";
import { useAppStore } from "../stores/app";
import { roomAPI } from "../api/rooms";
import { votingAPI, FIBONACCI_DECK } from "../api/voting";
import { WebSocketManager } from "../api/websocket";

const router = useRouter();
const route = useRoute();
const store = useAppStore();

const loading = ref(false);
const error = ref<string | null>(null);
const newRoundTitle = ref("");
const fibonacciDeck = FIBONACCI_DECK;

let wsManager: WebSocketManager | null = null;
let pollInterval: number | null = null;

onMounted(async () => {
  const roomCode = route.params.code as string;
  console.log("RoomView mounted. Room code:", roomCode);
  console.log("API URL:", import.meta.env.VITE_API_URL);
  console.log("Session token:", store.sessionToken ? "OK" : "MISSING");

  // Load room and participants
  try {
    const response = await roomAPI.getRoom(roomCode);
    console.log("Room loaded:", response.data);
    store.setRoom(response.data.room);
    store.setParticipants(response.data.participants);
  } catch (err: any) {
    console.error("Failed to load room:", err);
    error.value = "Failed to load room";
    setTimeout(() => router.push("/"), 2000);
    return;
  }

  // Fetch initial round state
  await fetchRoundState();

  // Connect WebSocket if we have a session token
  if (store.sessionToken) {
    wsManager = new WebSocketManager(roomCode, store.sessionToken);
    try {
      await wsManager.connect();
      console.log("WebSocket connected");
    } catch (err) {
      console.error("Failed to connect WebSocket:", err);
    }
  }

  // Set up polling to refresh room data every 3 seconds
  pollInterval = window.setInterval(async () => {
    try {
      // Refresh participants
      const response = await roomAPI.getRoom(roomCode);
      store.setParticipants(response.data.participants);

      // Refresh round state
      await fetchRoundState();
    } catch (err) {
      console.error("Polling error:", err);
    }
  }, 3000);
});

onUnmounted(() => {
  if (wsManager) {
    wsManager.disconnect();
  }
  if (pollInterval) {
    clearInterval(pollInterval);
  }
});

async function handleStartRound() {
  if (!newRoundTitle.value) {
    error.value = "Please enter a story title";
    return;
  }

  loading.value = true;
  error.value = null;

  try {
    console.log("Starting round with title:", newRoundTitle.value);
    const response = await votingAPI.startRound(
      store.roomCode,
      newRoundTitle.value,
    );
    console.log("Round started:", response.data);

    // Immediately set the round so UI updates
    store.setCurrentRound(response.data.round);
    store.setVotes([]); // Clear votes for new round
    newRoundTitle.value = "";

    // Small delay to ensure UI updates
    await new Promise((resolve) => setTimeout(resolve, 500));

    // Then fetch the actual state
    try {
      await fetchRoundState();
      console.log("Round state fetched successfully");
    } catch (fetchErr) {
      console.error(
        "Failed to fetch round state, but UI should be updated:",
        fetchErr,
      );
    }
  } catch (err: any) {
    console.error("Error starting round:", err);
    error.value =
      err.response?.data?.error || err.message || "Failed to start round";
  } finally {
    loading.value = false;
  }
}

async function handleVote(value: string) {
  if (!store.currentRound) return;

  loading.value = true;
  error.value = null;

  try {
    console.log("Casting vote:", value);
    await votingAPI.castVote(store.roomCode, store.currentRound.id, value);
    console.log("Vote cast successfully");

    // Fetch updated votes
    await fetchRoundState();
  } catch (err: any) {
    console.error("Error casting vote:", err);
    error.value =
      err.response?.data?.error || err.message || "Failed to cast vote";
  } finally {
    loading.value = false;
  }
}

async function handleRevealVotes() {
  if (!store.currentRound) return;

  loading.value = true;
  error.value = null;

  try {
    console.log("Revealing votes for round:", store.currentRound.id);
    await votingAPI.revealVotes(store.roomCode, store.currentRound.id);
    console.log("Votes revealed successfully");

    // Fetch updated round state
    await fetchRoundState();
  } catch (err: any) {
    console.error("Error revealing votes:", err);
    error.value =
      err.response?.data?.error || err.message || "Failed to reveal votes";
  } finally {
    loading.value = false;
  }
}

async function handleResetRound() {
  if (!store.currentRound) return;

  loading.value = true;
  error.value = null;

  try {
    console.log("Resetting round:", store.currentRound.id);
    await votingAPI.resetRound(store.roomCode, store.currentRound.id);
    store.setCurrentRound(null);
    store.setVotes([]);
    console.log("Round reset successfully");
  } catch (err: any) {
    console.error("Error resetting round:", err);
    error.value =
      err.response?.data?.error || err.message || "Failed to reset round";
  } finally {
    loading.value = false;
  }
}

async function fetchRoundState() {
  try {
    console.log("Fetching round state for room:", store.roomCode);
    const response = await votingAPI.getRoundState(store.roomCode);
    console.log("Round state:", response.data);
    store.setCurrentRound(response.data.round);
    store.setVotes(response.data.votes || []);
  } catch (err: any) {
    console.error("Failed to fetch round state:", err);
    error.value = err.response?.data?.error || "Failed to fetch round state";
  }
}

function handleLeaveRoom() {
  if (confirm("Are you sure you want to leave this room?")) {
    store.clearSession();
    router.push("/");
  }
}
</script>
