// Shared API types — mirrors the backend contract (see SPEC).

export interface Room {
  id: string;
  code: string;
  name: string;
  created_at: string;
}

export interface Participant {
  id: string;
  room_id: string;
  display_name: string;
  is_host: boolean;
  joined_at: string;
  online?: boolean;
}

export type RoundStatus = "voting" | "revealed";

export interface Round {
  id: string;
  room_id: string;
  story_title: string;
  status: RoundStatus;
  created_at: string;
}

export interface Vote {
  id: string;
  round_id: string;
  participant_id: string;
  /** Empty string while the round is voting (except the requester's own vote). */
  value: string;
  voted_at: string;
}

export interface RevealedVote {
  participant_id: string;
  display_name: string;
  value: string;
}

export interface SessionResponse {
  room: Room;
  session_token: string;
  participant: Participant;
}

export interface RoomDetailsResponse {
  room: Room;
  participants: Participant[];
}

export interface CurrentRoundResponse {
  round: Round | null;
  votes: Vote[];
}

export interface ApiErrorBody {
  error: string;
}

// ---- WebSocket events (server -> client) ----

export interface WSEventMap {
  participant_joined: { participant: Participant };
  participant_left: { participant_id: string; new_host_id: string | null };
  presence: { online: string[] };
  round_started: { round: Round };
  vote_cast: { participant_id: string };
  votes_revealed: { round_id: string; votes: RevealedVote[] };
  round_reset: { round: Round };
}

export type WSEventType = keyof WSEventMap;

export type WSEvent = {
  [K in WSEventType]: { type: K; payload: WSEventMap[K] };
}[WSEventType];

export const WS_EVENT_TYPES: readonly WSEventType[] = [
  "participant_joined",
  "participant_left",
  "presence",
  "round_started",
  "vote_cast",
  "votes_revealed",
  "round_reset",
];
