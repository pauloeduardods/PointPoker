import { AxiosError, AxiosHeaders, type AxiosResponse } from "axios";
import type { Participant, Room, Round, Vote } from "../../api/types";

export const room: Room = {
  id: "room-1",
  code: "ABC123",
  name: "Sprint planning",
  created_at: "2026-01-01T00:00:00Z",
};

export const host: Participant = {
  id: "p-host",
  room_id: room.id,
  display_name: "Alice",
  is_host: true,
  joined_at: "2026-01-01T00:00:00Z",
  online: true,
};

export const guest: Participant = {
  id: "p-guest",
  room_id: room.id,
  display_name: "Bob",
  is_host: false,
  joined_at: "2026-01-01T00:01:00Z",
  online: false,
};

export function makeRound(overrides: Partial<Round> = {}): Round {
  return {
    id: "round-1",
    room_id: room.id,
    story_title: "Login form",
    status: "voting",
    created_at: "2026-01-01T00:02:00Z",
    ...overrides,
  };
}

export function makeVote(participantId: string, value: string, roundId = "round-1"): Vote {
  return {
    id: `vote-${participantId}`,
    round_id: roundId,
    participant_id: participantId,
    value,
    voted_at: "2026-01-01T00:03:00Z",
  };
}

export function apiError(status: number, error?: string): AxiosError {
  const headers = new AxiosHeaders();
  const response: AxiosResponse = {
    status,
    statusText: "",
    headers,
    config: { headers },
    data: error === undefined ? {} : { error },
  };
  return new AxiosError(`Request failed with status ${status}`, "ERR_BAD_REQUEST", { headers }, undefined, response);
}

export function networkError(): AxiosError {
  return new AxiosError("Network Error", "ERR_NETWORK", { headers: new AxiosHeaders() });
}

/** Resolve pending promise callbacks. */
export async function flushPromises(times = 5): Promise<void> {
  for (let i = 0; i < times; i++) await Promise.resolve();
}
