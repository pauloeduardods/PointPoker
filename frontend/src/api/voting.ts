import client from "./client";
import type {
  CurrentRoundResponse,
  RevealedVote,
  Round,
  Vote,
} from "./types";

export type { Round, Vote, RevealedVote } from "./types";

const enc = (s: string) => encodeURIComponent(s);
const roomPath = (code: string) => `/rooms/${enc(code.trim().toUpperCase())}`;

export const votingAPI = {
  async startRound(code: string, storyTitle: string): Promise<Round> {
    const { data } = await client.post<{ round: Round }>(
      `${roomPath(code)}/rounds`,
      { story_title: storyTitle },
    );
    return data.round;
  },

  async getCurrentRound(code: string): Promise<CurrentRoundResponse> {
    const { data } = await client.get<CurrentRoundResponse>(
      `${roomPath(code)}/rounds/current`,
    );
    return { round: data.round ?? null, votes: data.votes ?? [] };
  },

  async castVote(code: string, roundId: string, value: string): Promise<Vote> {
    const { data } = await client.post<{ vote: Vote }>(
      `${roomPath(code)}/rounds/${enc(roundId)}/vote`,
      { value },
    );
    return data.vote;
  },

  async reveal(code: string, roundId: string): Promise<RevealedVote[]> {
    const { data } = await client.post<{ votes: RevealedVote[] }>(
      `${roomPath(code)}/rounds/${enc(roundId)}/reveal`,
    );
    return data.votes ?? [];
  },

  async reset(code: string, roundId: string): Promise<Round> {
    const { data } = await client.post<{ round: Round }>(
      `${roomPath(code)}/rounds/${enc(roundId)}/reset`,
    );
    return data.round;
  },
};
