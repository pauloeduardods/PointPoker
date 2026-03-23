import client from "./client";

export interface Room {
  id: string;
  code: string;
  name: string;
  status: "waiting" | "voting" | "revealed";
  created_at: string;
}

export interface Participant {
  id: string;
  room_id: string;
  display_name: string;
  is_host: boolean;
  joined_at: string;
}

export interface CreateRoomResponse {
  room: Room;
  session_token: string;
  participant: Participant;
}

export interface JoinRoomResponse {
  room: Room;
  session_token: string;
  participant: Participant;
}

export interface GetRoomResponse {
  room: Room;
  participants: Participant[];
}

export const roomAPI = {
  createRoom: (name: string, displayName: string) =>
    client.post<CreateRoomResponse>("/rooms", {
      name,
      display_name: displayName,
    }),

  getRoom: (code: string) => client.get<GetRoomResponse>(`/rooms/${code}`),

  joinRoom: (code: string, displayName: string) =>
    client.post<JoinRoomResponse>(`/rooms/${code}/join`, {
      display_name: displayName,
    }),
};
