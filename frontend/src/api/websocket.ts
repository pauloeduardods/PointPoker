import { useAppStore } from "../stores/app";

interface WSMessage {
  type: string;
  payload: Record<string, any>;
}

export class WebSocketManager {
  private ws: WebSocket | null = null;
  private url: string;
  // private token: string;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 3000;

  constructor(roomCode: string, token: string) {
    const wsProtocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsHost =
      import.meta.env.VITE_WS_URL || `${window.location.hostname}:8080`;
    this.url = `${wsProtocol}//${wsHost}/api/rooms/${roomCode}/ws?token=${token}`;
    // this.token = token;
  }

  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(this.url);

        this.ws.onopen = () => {
          const store = useAppStore();
          store.setWsConnected(true);
          this.reconnectAttempts = 0;
          resolve();
        };

        this.ws.onmessage = (event) => {
          try {
            const message: WSMessage = JSON.parse(event.data);
            this.handleMessage(message);
          } catch (e) {
            console.error("Failed to parse WebSocket message:", e);
          }
        };

        this.ws.onerror = (event) => {
          console.error("WebSocket error:", event);
          reject(new Error("WebSocket connection failed"));
        };

        this.ws.onclose = () => {
          const store = useAppStore();
          store.setWsConnected(false);
          this.attemptReconnect();
        };
      } catch (e) {
        reject(e);
      }
    });
  }

  private handleMessage(message: WSMessage) {
    const store = useAppStore();

    switch (message.type) {
      case "user_joined":
        console.log("User joined:", message.payload);
        break;

      case "user_left":
        console.log("User left:", message.payload);
        store.removeParticipant(message.payload.participant_id);
        break;

      case "round_started":
        console.log("Round started:", message.payload);
        break;

      case "vote_cast":
        console.log("Vote received:", message.payload);
        break;

      case "votes_revealed":
        console.log("Votes revealed:", message.payload);
        break;

      default:
        console.log("Unknown message type:", message.type);
    }
  }

  private attemptReconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      console.log(
        `Attempting to reconnect... (${this.reconnectAttempts}/${this.maxReconnectAttempts})`,
      );

      setTimeout(() => {
        this.connect().catch((e) => {
          console.error("Reconnection failed:", e);
        });
      }, this.reconnectDelay);
    }
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }
}
