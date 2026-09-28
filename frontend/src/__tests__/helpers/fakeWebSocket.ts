import type { SocketLike } from "../../api/websocket";
import type { WSEventMap, WSEventType } from "../../api/types";

export class FakeWebSocket implements SocketLike {
  static instances: FakeWebSocket[] = [];

  static reset(): void {
    FakeWebSocket.instances = [];
  }

  static latest(): FakeWebSocket {
    const ws = FakeWebSocket.instances.at(-1);
    if (!ws) throw new Error("no FakeWebSocket created");
    return ws;
  }

  onopen: ((ev: Event) => unknown) | null = null;
  onclose: ((ev: CloseEvent) => unknown) | null = null;
  onerror: ((ev: Event) => unknown) | null = null;
  onmessage: ((ev: MessageEvent) => unknown) | null = null;
  closeCalls = 0;
  readonly url: string;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  close(): void {
    this.closeCalls++;
  }

  // --- test controls ---
  serverOpen(): void {
    this.onopen?.(new Event("open"));
  }

  serverClose(): void {
    this.onerror?.(new Event("error"));
    this.onclose?.(new CloseEvent("close"));
  }

  emit<K extends WSEventType>(type: K, payload: WSEventMap[K]): void {
    this.sendRaw(JSON.stringify({ type, payload }));
  }

  sendRaw(data: string): void {
    this.onmessage?.(new MessageEvent("message", { data }));
  }
}
