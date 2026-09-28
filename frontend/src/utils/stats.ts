export const DECK = [
  "0",
  "1",
  "2",
  "3",
  "5",
  "8",
  "13",
  "21",
  "?",
  "☕",
] as const;

export type DeckValue = (typeof DECK)[number];

export interface DistributionEntry {
  value: string;
  count: number;
  /** Share of all cast votes, 0..100. */
  percent: number;
}

export interface RoundResults {
  total: number;
  distribution: DistributionEntry[];
  /** Average of numeric votes (`?` and `☕` excluded), or null if none. */
  average: number | null;
  /** True when there is at least one numeric vote and all numeric votes are equal. */
  consensus: boolean;
}

export function isNumericVote(value: string): boolean {
  return /^\d+(\.\d+)?$/.test(value);
}

function deckIndex(value: string): number {
  const i = (DECK as readonly string[]).indexOf(value);
  return i === -1 ? DECK.length : i;
}

export function computeResults(values: readonly string[]): RoundResults {
  const cast = values.filter((v) => v !== "");
  const counts = new Map<string, number>();
  for (const v of cast) counts.set(v, (counts.get(v) ?? 0) + 1);

  const total = cast.length;
  const distribution: DistributionEntry[] = [...counts.entries()]
    .map(([value, count]) => ({
      value,
      count,
      percent: total === 0 ? 0 : (count / total) * 100,
    }))
    .sort((a, b) => deckIndex(a.value) - deckIndex(b.value) || a.value.localeCompare(b.value));

  const numeric = cast.filter(isNumericVote).map(Number);
  const average =
    numeric.length === 0
      ? null
      : numeric.reduce((sum, n) => sum + n, 0) / numeric.length;
  const consensus = numeric.length > 0 && numeric.every((n) => n === numeric[0]);

  return { total, distribution, average, consensus };
}

export function formatAverage(avg: number | null): string {
  if (avg === null) return "—";
  return Number.isInteger(avg) ? String(avg) : avg.toFixed(1);
}
