import client from "./client";

export interface VotingRound {
  id: string;
  room_id: string;
  story_title: string;
  status: "voting" | "revealed";
  created_at: string;
}

export interface Vote {
  id: string;
  round_id: string;
  participant_id: string;
  value: string;
  voted_at: string;
}

export const FIBONACCI_DECK = ["0", "1", "2", "3", "5", "8", "13", "21", "?"];

export const votingAPI = {
  startRound: (code: string, storyTitle: string) =>
    client.post<{ round: VotingRound }>(`/rooms/${code}/rounds`, {
      story_title: storyTitle,
    }),

  getRoundState: (code: string) =>
    client.get<{ round: VotingRound; votes: Vote[] }>(
      `/rooms/${code}/rounds/current`,
    ),

  castVote: (code: string, roundId: string, value: string) =>
    client.post(`/rooms/${code}/rounds/${roundId}/vote`, {
      value,
    }),

  revealVotes: (code: string, roundId: string) =>
    client.post(`/rooms/${code}/rounds/${roundId}/reveal`),

  resetRound: (code: string, roundId: string) =>
    client.post(`/rooms/${code}/rounds/${roundId}/reset`),
};
