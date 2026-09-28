<template>
  <div
    class="flex min-h-screen items-center justify-center bg-gradient-to-br from-indigo-600 via-indigo-500 to-violet-600 p-4"
  >
    <main class="w-full max-w-md">
      <div class="mb-6 text-center text-white">
        <div
          class="mx-auto mb-3 flex h-14 w-14 items-center justify-center rounded-2xl bg-white/15 text-2xl font-bold shadow-lg ring-1 ring-white/30"
          aria-hidden="true"
        >
          5
        </div>
        <h1 class="text-3xl font-bold tracking-tight">Point Poker</h1>
        <p class="mt-1 text-indigo-100">Real-time estimation for agile teams</p>
      </div>

      <div class="rounded-2xl bg-white p-6 shadow-2xl sm:p-8">
        <div
          class="mb-6 grid grid-cols-2 gap-1 rounded-lg bg-slate-100 p-1"
          role="tablist"
          aria-label="Create or join a room"
        >
          <button
            v-for="tab in tabs"
            :id="`tab-${tab.id}`"
            :key="tab.id"
            type="button"
            role="tab"
            :aria-selected="activeTab === tab.id"
            :aria-controls="`panel-${tab.id}`"
            :data-testid="`tab-${tab.id}`"
            class="rounded-md px-3 py-2 text-sm font-semibold transition focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
            :class="
              activeTab === tab.id
                ? 'bg-white text-indigo-700 shadow-sm'
                : 'text-slate-600 hover:text-slate-900'
            "
            @click="selectTab(tab.id)"
          >
            {{ tab.label }}
          </button>
        </div>

        <p
          v-if="invitedCode && activeTab === 'join'"
          class="mb-4 rounded-lg bg-indigo-50 px-3 py-2 text-sm text-indigo-800"
          data-testid="invite-hint"
        >
          You've been invited to room
          <strong class="font-mono">{{ invitedCode }}</strong>. Enter your name to join.
        </p>

        <form
          v-if="activeTab === 'create'"
          id="panel-create"
          role="tabpanel"
          aria-labelledby="tab-create"
          class="space-y-4"
          data-testid="create-form"
          @submit.prevent="handleCreate"
        >
          <div>
            <label for="create-room-name" class="label">Room name</label>
            <input
              id="create-room-name"
              v-model="createForm.roomName"
              type="text"
              class="input"
              placeholder="e.g. Sprint 42 planning"
              maxlength="100"
              autocomplete="off"
              required
            />
          </div>
          <div>
            <label for="create-display-name" class="label">Your name</label>
            <input
              id="create-display-name"
              v-model="createForm.displayName"
              type="text"
              class="input"
              placeholder="e.g. Ada Lovelace"
              maxlength="50"
              autocomplete="nickname"
              required
            />
          </div>
          <button
            type="submit"
            class="btn-primary w-full py-2.5"
            :disabled="!canCreate || busy"
          >
            {{ busy ? "Creating…" : "Create room" }}
          </button>
        </form>

        <form
          v-else
          id="panel-join"
          role="tabpanel"
          aria-labelledby="tab-join"
          class="space-y-4"
          data-testid="join-form"
          @submit.prevent="handleJoin"
        >
          <div>
            <label for="join-room-code" class="label">Room code</label>
            <input
              id="join-room-code"
              v-model="joinForm.roomCode"
              type="text"
              class="input font-mono uppercase tracking-widest"
              placeholder="ABC123"
              autocomplete="off"
              autocapitalize="characters"
              spellcheck="false"
              required
            />
          </div>
          <div>
            <label for="join-display-name" class="label">Your name</label>
            <input
              id="join-display-name"
              ref="joinNameInput"
              v-model="joinForm.displayName"
              type="text"
              class="input"
              placeholder="e.g. Grace Hopper"
              maxlength="50"
              autocomplete="nickname"
              required
            />
          </div>
          <button
            type="submit"
            class="btn-primary w-full py-2.5"
            :disabled="!canJoin || busy"
          >
            {{ busy ? "Joining…" : "Join room" }}
          </button>
        </form>

        <p
          v-if="error"
          role="alert"
          class="mt-4 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700"
        >
          {{ error }}
        </p>

        <div
          v-if="resumeCode"
          class="mt-6 border-t border-slate-200 pt-4 text-center text-sm text-slate-600"
        >
          <router-link
            :to="{ name: 'room', params: { code: resumeCode } }"
            class="font-semibold text-indigo-600 hover:text-indigo-800 focus:outline-none focus-visible:underline"
            data-testid="resume-link"
          >
            Return to room {{ resumeCode }} →
          </router-link>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { roomAPI } from "../api/rooms";
import { friendlyError } from "../api/client";
import type { SessionResponse } from "../api/types";
import { useAppStore } from "../stores/app";
import { loadSession, normalizeRoomCode } from "../utils/session";

type Tab = "create" | "join";

const tabs: { id: Tab; label: string }[] = [
  { id: "create", label: "Create room" },
  { id: "join", label: "Join room" },
];

const route = useRoute();
const router = useRouter();
const store = useAppStore();

const activeTab = ref<Tab>("create");
const busy = ref(false);
const error = ref<string | null>(null);
const joinNameInput = ref<HTMLInputElement | null>(null);

const createForm = reactive({ roomName: "", displayName: "" });
const joinForm = reactive({ roomCode: "", displayName: "" });

const invitedCode = ref<string | null>(null);
const resumeCode = computed(() => {
  const s = store.session ?? loadSession();
  return s && s.roomCode !== invitedCode.value ? s.roomCode : null;
});

const canCreate = computed(
  () => createForm.roomName.trim() !== "" && createForm.displayName.trim() !== "",
);
const canJoin = computed(
  () => joinForm.roomCode.trim() !== "" && joinForm.displayName.trim() !== "",
);

watch(
  () => route.query.join,
  (value) => {
    const raw = Array.isArray(value) ? value[0] : value;
    if (typeof raw === "string" && raw.trim() !== "") {
      const code = normalizeRoomCode(raw);
      invitedCode.value = code;
      joinForm.roomCode = code;
      activeTab.value = "join";
      void nextTick(() => joinNameInput.value?.focus());
    }
  },
  { immediate: true },
);

function selectTab(tab: Tab) {
  activeTab.value = tab;
  error.value = null;
}

async function enter(request: () => Promise<SessionResponse>) {
  busy.value = true;
  error.value = null;
  try {
    const res = await request();
    store.startSession(res);
    await router.push({ name: "room", params: { code: res.room.code } });
  } catch (err) {
    error.value = friendlyError(err);
  } finally {
    busy.value = false;
  }
}

function handleCreate() {
  if (!canCreate.value) {
    error.value = "Please fill in all fields.";
    return;
  }
  return enter(() =>
    roomAPI.createRoom(createForm.roomName.trim(), createForm.displayName.trim()),
  );
}

function handleJoin() {
  if (!canJoin.value) {
    error.value = "Please fill in all fields.";
    return;
  }
  return enter(() =>
    roomAPI.joinRoom(
      normalizeRoomCode(joinForm.roomCode),
      joinForm.displayName.trim(),
    ),
  );
}
</script>
