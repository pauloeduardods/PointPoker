<template>
  <section class="card" aria-labelledby="participants-heading">
    <div class="mb-4 flex items-baseline justify-between">
      <h2 id="participants-heading" class="text-base font-semibold text-slate-900">
        Participants
        <span class="ml-1 text-sm font-normal text-slate-500">({{ participants.length }})</span>
      </h2>
      <span
        v-if="showVotes"
        class="text-sm font-medium text-slate-600"
        data-testid="voted-count"
      >
        {{ votedCount }}/{{ participants.length }} voted
      </span>
    </div>

    <ul class="space-y-2">
      <li
        v-for="p in sorted"
        :key="p.id"
        class="flex items-center gap-3 rounded-lg px-3 py-2"
        :class="p.id === myId ? 'bg-indigo-50' : 'bg-slate-50'"
        :data-testid="`participant-${p.id}`"
      >
        <span
          class="h-2.5 w-2.5 flex-none rounded-full"
          :class="isOnline(p.id) ? 'bg-emerald-500' : 'bg-slate-300'"
          :title="isOnline(p.id) ? 'Online' : 'Offline'"
          :data-online="isOnline(p.id)"
          data-testid="online-dot"
        >
          <span class="sr-only">{{ isOnline(p.id) ? "Online" : "Offline" }}</span>
        </span>

        <span class="min-w-0 flex-1 truncate text-sm font-medium text-slate-800">
          {{ p.display_name }}
          <span v-if="p.id === myId" class="font-normal text-slate-500">(you)</span>
        </span>

        <span
          v-if="p.is_host"
          class="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-800"
        >
          Host
        </span>

        <template v-if="showVotes">
          <span
            v-if="hasVoted(p.id)"
            class="flex h-5 w-5 items-center justify-center rounded-full bg-emerald-500 text-xs text-white"
            data-testid="voted-mark"
          >
            <span aria-hidden="true">✓</span>
            <span class="sr-only">Voted</span>
          </span>
          <span
            v-else
            class="h-5 w-5 rounded-full border-2 border-dashed border-slate-300"
            title="Not voted yet"
          >
            <span class="sr-only">Not voted yet</span>
          </span>
        </template>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { Participant } from "../api/types";

const props = defineProps<{
  participants: Participant[];
  myId: string | null;
  onlineIds: Set<string>;
  votedIds: Set<string>;
  votedCount: number;
  showVotes: boolean;
}>();

const sorted = computed(() =>
  [...props.participants].sort((a, b) => {
    if (a.is_host !== b.is_host) return a.is_host ? -1 : 1;
    return a.joined_at.localeCompare(b.joined_at);
  }),
);

function isOnline(id: string) {
  return props.onlineIds.has(id);
}

function hasVoted(id: string) {
  return props.votedIds.has(id);
}
</script>
