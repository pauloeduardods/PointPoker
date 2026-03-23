<template>
  <div
    class="min-h-screen bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center p-4"
  >
    <div class="bg-white rounded-lg shadow-2xl p-8 w-full max-w-md">
      <h1 class="text-4xl font-bold text-center mb-2 text-gray-800">
        Point Poker
      </h1>
      <p class="text-center text-gray-600 mb-8">Agile estimation made easy</p>

      <div class="space-y-4">
        <!-- Create Room Tab -->
        <div v-if="activeTab === 'create'" class="space-y-4">
          <h2 class="text-2xl font-semibold text-gray-800">Create Room</h2>

          <div>
            <label class="block text-sm font-medium text-gray-700 mb-2"
              >Room Name</label
            >
            <input
              v-model="createForm.roomName"
              type="text"
              placeholder="e.g., Sprint Planning"
              class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              @keyup.enter="handleCreateRoom"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 mb-2"
              >Your Name</label
            >
            <input
              v-model="createForm.displayName"
              type="text"
              placeholder="e.g., John Doe"
              class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              @keyup.enter="handleCreateRoom"
            />
          </div>

          <button
            @click="handleCreateRoom"
            :disabled="
              !createForm.roomName || !createForm.displayName || loading
            "
            class="w-full bg-blue-500 hover:bg-blue-600 disabled:bg-gray-400 text-white font-semibold py-2 px-4 rounded-lg transition"
          >
            <span v-if="loading">Creating...</span>
            <span v-else>Create Room</span>
          </button>

          <div
            class="text-center text-sm text-gray-600 pt-4 border-t border-gray-200"
          >
            <button
              @click="activeTab = 'join'"
              class="text-blue-500 hover:text-blue-700 font-semibold"
            >
              Join existing room?
            </button>
          </div>
        </div>

        <!-- Join Room Tab -->
        <div v-if="activeTab === 'join'" class="space-y-4">
          <h2 class="text-2xl font-semibold text-gray-800">Join Room</h2>

          <div>
            <label class="block text-sm font-medium text-gray-700 mb-2"
              >Room Code</label
            >
            <input
              v-model="joinForm.roomCode"
              type="text"
              placeholder="e.g., ABC123"
              class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 uppercase"
              @keyup.enter="handleJoinRoom"
            />
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 mb-2"
              >Your Name</label
            >
            <input
              v-model="joinForm.displayName"
              type="text"
              placeholder="e.g., John Doe"
              class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              @keyup.enter="handleJoinRoom"
            />
          </div>

          <button
            @click="handleJoinRoom"
            :disabled="!joinForm.roomCode || !joinForm.displayName || loading"
            class="w-full bg-green-500 hover:bg-green-600 disabled:bg-gray-400 text-white font-semibold py-2 px-4 rounded-lg transition"
          >
            <span v-if="loading">Joining...</span>
            <span v-else>Join Room</span>
          </button>

          <div
            class="text-center text-sm text-gray-600 pt-4 border-t border-gray-200"
          >
            <button
              @click="activeTab = 'create'"
              class="text-blue-500 hover:text-blue-700 font-semibold"
            >
              Create new room?
            </button>
          </div>
        </div>
      </div>

      <!-- Error Message -->
      <div
        v-if="error"
        class="mt-4 p-4 bg-red-50 border border-red-200 rounded-lg"
      >
        <p class="text-red-700 text-sm">{{ error }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { useAppStore } from "../stores/app";
import { roomAPI } from "../api/rooms";

const router = useRouter();
const store = useAppStore();

const activeTab = ref<"create" | "join">("create");
const loading = ref(false);
const error = ref<string | null>(null);

const createForm = ref({
  roomName: "",
  displayName: "",
});

const joinForm = ref({
  roomCode: "",
  displayName: "",
});

async function handleCreateRoom() {
  if (!createForm.value.roomName || !createForm.value.displayName) {
    error.value = "Please fill in all fields";
    return;
  }

  loading.value = true;
  error.value = null;

  try {
    console.log("Creating room:", createForm.value);
    const response = await roomAPI.createRoom(
      createForm.value.roomName,
      createForm.value.displayName,
    );
    console.log("Room created:", response.data);

    store.setRoom(response.data.room);
    store.setParticipant(response.data.participant);
    store.setSessionToken(response.data.session_token);

    console.log("Navigating to room:", response.data.room.code);
    router.push(`/room/${response.data.room.code}`);
  } catch (err: any) {
    console.error("Error creating room:", err);
    error.value =
      err.response?.data?.error || err.message || "Failed to create room";
  } finally {
    loading.value = false;
  }
}

async function handleJoinRoom() {
  if (!joinForm.value.roomCode || !joinForm.value.displayName) {
    error.value = "Please fill in all fields";
    return;
  }

  loading.value = true;
  error.value = null;

  try {
    console.log("Joining room:", joinForm.value.roomCode);
    const response = await roomAPI.joinRoom(
      joinForm.value.roomCode.toUpperCase(),
      joinForm.value.displayName,
    );
    console.log("Room joined:", response.data);

    store.setRoom(response.data.room);
    store.setParticipant(response.data.participant);
    store.setSessionToken(response.data.session_token);

    console.log("Navigating to room:", response.data.room.code);
    router.push(`/room/${response.data.room.code}`);
  } catch (err: any) {
    console.error("Error joining room:", err);
    error.value =
      err.response?.data?.error || err.message || "Failed to join room";
  } finally {
    loading.value = false;
  }
}
</script>
