<template>
  <section aria-labelledby="results-heading" data-testid="results">
    <h3 id="results-heading" class="sr-only">Results</h3>

    <div
      v-if="results.consensus"
      class="mb-4 flex items-center gap-2 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-emerald-800"
      role="status"
      data-testid="consensus"
    >
      <span aria-hidden="true" class="text-xl">🎉</span>
      <span class="font-semibold">Consensus!</span>
      <span class="text-sm">Everyone agreed on {{ formatAverage(results.average) }}.</span>
    </div>

    <dl class="mb-6 grid grid-cols-2 gap-3">
      <div class="rounded-lg bg-slate-50 p-4 text-center">
        <dt class="text-xs font-semibold uppercase tracking-wide text-slate-500">Average</dt>
        <dd class="mt-1 text-3xl font-bold text-indigo-700" data-testid="average">
          {{ formatAverage(results.average) }}
        </dd>
      </div>
      <div class="rounded-lg bg-slate-50 p-4 text-center">
        <dt class="text-xs font-semibold uppercase tracking-wide text-slate-500">Votes</dt>
        <dd class="mt-1 text-3xl font-bold text-slate-800">{{ results.total }}</dd>
      </div>
    </dl>

    <h4 class="mb-2 text-sm font-semibold text-slate-700">Distribution</h4>
    <p v-if="results.total === 0" class="mb-6 text-sm text-slate-500">Nobody voted in this round.</p>
    <ul v-else class="mb-6 space-y-2" data-testid="distribution">
      <li v-for="d in results.distribution" :key="d.value" class="flex items-center gap-3">
        <span class="w-8 text-right text-lg font-bold text-slate-800">{{ d.value }}</span>
        <div
          class="h-6 flex-1 overflow-hidden rounded-full bg-slate-100"
          role="img"
          :aria-label="`${d.count} ${d.count === 1 ? 'vote' : 'votes'} for ${d.value}`"
        >
          <div
            class="h-full rounded-full bg-indigo-500 transition-all duration-500"
            :style="{ width: `${Math.max(d.percent, 4)}%` }"
          />
        </div>
        <span class="w-16 text-right text-sm text-slate-600">
          {{ d.count }} {{ d.count === 1 ? "vote" : "votes" }}
        </span>
      </li>
    </ul>

    <h4 class="mb-2 text-sm font-semibold text-slate-700">Individual votes</h4>
    <ul class="flex flex-wrap gap-3" data-testid="individual-votes">
      <li
        v-for="entry in entries"
        :key="entry.id"
        class="flex w-20 flex-col items-center gap-1"
      >
        <span
          class="flex aspect-[3/4] w-14 items-center justify-center rounded-lg border-2 text-lg font-bold"
          :class="
            entry.value
              ? 'border-indigo-600 bg-white text-indigo-700'
              : 'border-dashed border-slate-300 bg-slate-50 text-slate-400'
          "
        >
          {{ entry.value || "–" }}
        </span>
        <span class="w-full truncate text-center text-xs text-slate-600" :title="entry.name">
          {{ entry.name }}
        </span>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { Participant, Vote } from "../api/types";
import { formatAverage, type RoundResults } from "../utils/stats";

const props = defineProps<{
  results: RoundResults;
  votes: Vote[];
  participants: Participant[];
}>();

interface Entry {
  id: string;
  name: string;
  value: string;
}

const entries = computed<Entry[]>(() => {
  const byId = new Map(props.votes.map((v) => [v.participant_id, v.value]));
  const list: Entry[] = props.participants.map((p) => ({
    id: p.id,
    name: p.display_name,
    value: byId.get(p.id) ?? "",
  }));
  // Votes from people who have since left the room.
  for (const v of props.votes) {
    if (!props.participants.some((p) => p.id === v.participant_id)) {
      list.push({ id: v.participant_id, name: "Former participant", value: v.value });
    }
  }
  return list.sort((a, b) => Number(b.value !== "") - Number(a.value !== ""));
});
</script>
