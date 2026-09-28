import client from "./client";
import type {
  Participant,
  RoomDetailsResponse,
  SessionResponse,
} from "./types";

export type { Room, Participant } from "./types";

const enc = (code: string) => encodeURIComponent(code.trim().toUpperCase());

export const roomAPI = {
  async createRoom(name: string, displayName: string): Promise<SessionResponse> {
    const { data } = await client.post<SessionResponse>("/rooms", {
      name,
      display_name: displayName,
    });
    return data;
  },

  async getRoom(code: string): Promise<RoomDetailsResponse> {
    const { data } = await client.get<RoomDetailsResponse>(`/rooms/${enc(code)}`);
    return data;
  },

  async joinRoom(code: string, displayName: string): Promise<SessionResponse> {
    const { data } = await client.post<SessionResponse>(
      `/rooms/${enc(code)}/join`,
      { display_name: displayName },
    );
    return data;
  },

  async getMe(code: string): Promise<Participant> {
    const { data } = await client.get<{ participant: Participant }>(
      `/rooms/${enc(code)}/me`,
    );
    return data.participant;
  },

  async leaveRoom(code: string): Promise<void> {
    await client.delete(`/rooms/${enc(code)}/participants/me`);
  },
};
