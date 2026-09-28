<template>
  <div class="min-h-screen bg-slate-50">
    <header class="sticky top-0 z-10 border-b border-slate-200 bg-white/90 backdrop-blur">
      <div
        class="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 py-3 sm:px-6"
      >
        <div class="min-w-0">
          <router-link
            to="/"
            class="text-xs font-semibold uppercase tracking-wide text-indigo-600 hover:text-indigo-800 focus:outline-none focus-visible:underline"
          >
            Point Poker
          </router-link>
          <h1 class="truncate text-lg font-bold text-slate-900 sm:text-xl" data-testid="room-name">
            {{ store.room?.name ?? "Loading room…" }}
          </h1>
          <div class="mt-0.5 flex flex-wrap items-center gap-2 text-sm text-slate-600">
            <span>
              Code
              <span class="rounded bg-slate-100 px-1.5 py-0.5 font-mono font-semibold tracking-widest text-slate-800" data-testid="room-code">{{ code }}</span>
            </span>
            <button
              type="button"
              class="rounded px-1.5 py-0.5 font-medium text-indigo-600 hover:bg-indigo-50 hover:text-indigo-800 focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500"
              data-testid="copy-link"
              @click="copyInvite"
            >
              {{ copyLabel }}
            </button>
            <span class="sr-only" role="status" aria-live="polite">{{ copyStatus }}</span>
          </div>
        </div>

        <div class="flex items-center gap-3">
          <span
            class="inline-flex items-center gap-1.5 text-xs font-medium"
            :class="store.wsConnected ? 'text-emerald-700' : 'text-amber-700'"
            data-testid="connection-status"
            role="status"
          >
            <span
              class="h-2 w-2 rounded-full"
              :class="store.wsConnected ? 'bg-emerald-500' : 'animate-pulse bg-amber-500'"
              aria-hidden="true"
            />
            {{ store.wsConnected ? "Live" : "Reconnecting…" }}
          </span>
          <button
            type="button"
            class="btn-danger"
            :disabled="busy === 'leave'"
            data-testid="leave"
            @click="leave"
          >
            {{ busy === "leave" ? "Leaving…" : "Leave" }}
          </button>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-6xl px-4 py-6 sm:px-6 sm:py-8">
      <div v-if="phase === 'loading'" class="py-24 text-center text-slate-500" role="status">
        Loading room…
      </div>

      <div v-else-if="phase === 'error'" class="card mx-auto max-w-md text-center" role="alert">
        <p class="mb-4 text-slate-700">{{ loadError }}</p>
        <div class="flex justify-center gap-2">
          <button type="button" class="btn-primary" @click="init">Try again</button>
          <router-link to="/" class="btn-secondary">Back home</router-link>
        </div>
      </div>

      <div v-else class="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div class="space-y-6 lg:col-span-2">
          <p
            v-if="actionError"
            role="alert"
            class="flex items-start justify-between gap-3 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
            data-testid="action-error"
          >
            <span>{{ actionError }}</span>
            <button
              type="button"
              class="rounded text-red-500 hover:text-red-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-500"
              aria-label="Dismiss error"
              @click="actionError = null"
            >
              <span aria-hidden="true">✕</span>
            </button>
          </p>

          <section class="card" aria-labelledby="round-heading">
            <template v-if="!round">
              <div class="py-6 text-center">
                <h2 id="round-heading" class="text-lg font-semibold text-slate-900">
                  No active round
                </h2>
                <p v-if="!store.isHost" class="mt-2 text-sm text-slate-500" data-testid="waiting-host">
                  Waiting for the host to start a round…
                </p>
                <p v-else class="mt-2 text-sm text-slate-500">
                  Enter a story to start estimating.
                </p>
              </div>
            </template>

            <template v-else>
              <div class="mb-6 flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="text-xs font-semibold uppercase tracking-wide text-slate-500">
                    Current story
                  </p>
                  <h2 id="round-heading" class="break-words text-xl font-bold text-slate-900" data-testid="story-title">
                    {{ round.story_title }}
                  </h2>
                </div>
                <span
                  class="rounded-full px-3 py-1 text-xs font-semibold"
                  :class="
                    round.status === 'voting'
                      ? 'bg-amber-100 text-amber-800'
                      : 'bg-emerald-100 text-emerald-800'
                  "
                  data-testid="round-status"
                >
                  {{ round.status === "voting" ? "Voting" : "Revealed" }}
                </span>
              </div>

              <template v-if="round.status === 'voting'">
                <VotingDeck :selected="store.myVote" @vote="onVote" />
                <p class="mt-4 text-sm text-slate-600" aria-live="polite" data-testid="vote-hint">
                  <template v-if="store.myVote">
                    Your vote: <strong>{{ store.myVote }}</strong> — pick another card to change it.
                  </template>
                  <template v-else>Choose a card. Votes stay hidden until the host reveals them.</template>
                </p>
              </template>

              <RoundResults
                v-else-if="store.results"
                :results="store.results"
                :votes="store.votes"
                :participants="store.participants"
              />
            </template>
          </section>

          <section
            v-if="store.isHost"
            class="card"
            aria-labelledby="host-heading"
            data-testid="host-controls"
          >
            <h2 id="host-heading" class="mb-4 text-base font-semibold text-slate-900">Host controls</h2>

            <div v-if="round" class="mb-4 flex flex-wrap gap-2">
              <button
                v-if="round.status === 'voting'"
                type="button"
                class="btn-primary"
                :disabled="busy !== null"
                data-testid="reveal"
                @click="reveal"
              >
                {{ busy === "reveal" ? "Revealing…" : "Reveal cards" }}
              </button>
              <button
                v-else
                type="button"
                class="btn-secondary"
                :disabled="busy !== null"
                data-testid="revote"
                @click="revote"
              >
                {{ busy === "revote" ? "Resetting…" : "Revote" }}
              </button>
              <button
                v-if="round.status === 'voting' && !showNextForm"
                type="button"
                class="btn-secondary"
                data-testid="skip-story"
                @click="openNextForm"
              >
                Next story
              </button>
            </div>

            <form
              v-if="nextFormVisible"
              class="flex flex-col gap-2 sm:flex-row sm:items-end"
              data-testid="next-story-form"
              @submit.prevent="startRound"
            >
              <div class="flex-1">
                <label for="story-title" class="label">
                  {{ round ? "Next story" : "First story" }}
                </label>
                <input
                  id="story-title"
                  ref="storyInput"
                  v-model="storyTitle"
                  type="text"
                  class="input"
                  placeholder="e.g. User can reset their password"
                  maxlength="200"
                  autocomplete="off"
                />
              </div>
              <button
                type="submit"
                class="btn-primary"
                :disabled="storyTitle.trim() === '' || busy !== null"
                data-testid="start-round"
              >
                {{ busy === "start" ? "Starting…" : "Start round" }}
              </button>
            </form>
          </section>
        </div>

        <aside>
          <ParticipantList
            :participants="store.participants"
            :my-id="store.myId"
            :online-ids="store.onlineIds"
            :voted-ids="store.votedIds"
            :voted-count="store.votedCount"
            :show-votes="round?.status === 'voting'"
          />
        </aside>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import ParticipantList from "../components/ParticipantList.vue";
import RoundResults from "../components/RoundResults.vue";
import VotingDeck from "../components/VotingDeck.vue";
import { errorStatus, friendlyError } from "../api/client";
import { useRoomSync } from "../composables/useRoomSync";
import { useAppStore } from "../stores/app";
import { copyText, inviteLink } from "../utils/clipboard";
import { normalizeRoomCode } from "../utils/session";

type Busy = "reveal" | "revote" | "start" | "leave" | null;

const route = useRoute();
const router = useRouter();
const store = useAppStore();
const sync = useRoomSync();

const code = computed(() => {
  const raw = route.params.code;
  return normalizeRoomCode(Array.isArray(raw) ? (raw[0] ?? "") : (raw ?? ""));
});
const round = computed(() => store.round);

const phase = ref<"loading" | "ready" | "error">("loading");
const loadError = ref<string | null>(null);
const actionError = ref<string | null>(null);
const busy = ref<Busy>(null);
const storyTitle = ref("");
const showNextForm = ref(false);
const storyInput = ref<HTMLInputElement | null>(null);
const copyLabel = ref("Copy invite link");
const copyStatus = ref("");

const nextFormVisible = computed(
  () => !round.value || round.value.status === "revealed" || showNextForm.value,
);

async function redirectToJoin() {
  sync.stop();
  await router.replace({ name: "home", query: { join: code.value } });
}

async function init() {
  phase.value = "loading";
  loadError.value = null;
  sync.stop();
  try {
    const result = await store.restoreSession(code.value);
    if (result === "invalid") {
      await redirectToJoin();
      return;
    }
    await store.refreshAll();
    phase.value = "ready";
    const token = store.sessionToken;
    if (token) sync.start(code.value, token);
  } catch (err) {
    loadError.value = friendlyError(err, "We couldn't load this room.");
    phase.value = "error";
  }
}

async function handleActionError(err: unknown) {
  const status = errorStatus(err);
  if (status === 401) {
    store.clearSession();
    await redirectToJoin();
    return;
  }
  actionError.value = friendlyError(err);
  if (status === 409 || status === 403) {
    store.refreshAll().catch(() => undefined);
  }
}

async function runAction(kind: Exclude<Busy, null>, action: () => Promise<void>) {
  busy.value = kind;
  actionError.value = null;
  try {
    await action();
  } catch (err) {
    await handleActionError(err);
  } finally {
    busy.value = null;
  }
}

function onVote(value: string) {
  actionError.value = null;
  store.castVote(value).catch(handleActionError);
}

function reveal() {
  return runAction("reveal", () => store.revealVotes());
}

function revote() {
  return runAction("revote", () => store.revote());
}

function startRound() {
  const title = storyTitle.value.trim();
  if (!title) return;
  return runAction("start", async () => {
    await store.startRound(title);
    storyTitle.value = "";
    showNextForm.value = false;
  });
}

function openNextForm() {
  showNextForm.value = true;
  void nextTick(() => storyInput.value?.focus());
}

async function leave() {
  if (!window.confirm("Leave this room? You can rejoin later with the room code.")) return;
  busy.value = "leave";
  sync.stop();
  await store.leaveRoom();
  busy.value = null;
  await router.push({ name: "home" });
}

let copyTimer: ReturnType<typeof setTimeout> | null = null;
async function copyInvite() {
  const ok = await copyText(inviteLink(code.value));
  copyLabel.value = ok ? "Link copied!" : "Copy failed";
  copyStatus.value = ok ? "Invite link copied to clipboard" : "Could not copy the invite link";
  if (copyTimer) clearTimeout(copyTimer);
  copyTimer = setTimeout(() => {
    copyLabel.value = "Copy invite link";
    copyStatus.value = "";
  }, 2000);
}

onMounted(init);
onBeforeUnmount(() => {
  if (copyTimer) clearTimeout(copyTimer);
});
watch(code, (next, prev) => {
  if (next && next !== prev && route.name === "room") void init();
});
</script>
