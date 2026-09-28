<template>
  <fieldset>
    <legend class="mb-3 text-sm font-semibold text-slate-700">
      Pick your estimate
    </legend>
    <div class="grid grid-cols-5 gap-2 sm:gap-3">
      <button
        v-for="value in DECK"
        :key="value"
        type="button"
        class="aspect-[3/4] rounded-xl border-2 text-xl font-bold shadow-sm transition focus:outline-none focus-visible:ring-4 focus-visible:ring-indigo-300 sm:text-2xl"
        :class="
          value === selected
            ? '-translate-y-1 border-indigo-600 bg-indigo-600 text-white shadow-md'
            : 'border-slate-200 bg-white text-slate-700 hover:-translate-y-0.5 hover:border-indigo-300'
        "
        :aria-pressed="value === selected"
        :aria-label="cardLabel(value)"
        :disabled="disabled"
        :data-testid="`card-${value}`"
        @click="emit('vote', value)"
      >
        {{ value }}
      </button>
    </div>
  </fieldset>
</template>

<script setup lang="ts">
import { DECK } from "../utils/stats";

defineProps<{ selected: string | null; disabled?: boolean }>();
const emit = defineEmits<{ vote: [value: string] }>();

function cardLabel(value: string): string {
  if (value === "?") return "Vote: not sure";
  if (value === "☕") return "Vote: need a break";
  return `Vote ${value}`;
}
</script>
