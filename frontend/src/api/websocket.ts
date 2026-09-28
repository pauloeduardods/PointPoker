import {
  WS_EVENT_TYPES,
  type WSEvent,
  type WSEventMap,
  type WSEventType,
} from "./types";

export type ConnectionStatus = "connecting" | "open" | "closed";

/** Minimal subset of the browser WebSocket we rely on (lets tests inject a fake). */
export interface SocketLike {
  onopen: ((ev: Event) => unknown) | null;
  onclose: ((ev: CloseEvent) => unknown) | null;
  onerror: ((ev: Event) => unknown) | null;
  onmessage: ((ev: MessageEvent) => unknown) | null;
  close(code?: number, reason?: string): void;
}

export type SocketFactory = (url: string) => SocketLike;

export interface WebSocketManagerOptions {
  onEvent?: (event: WSEvent) => void;
  onStatusChange?: (status: ConnectionStatus) => void;
  /** First reconnect delay; doubles each failed attempt. */
  baseDelayMs?: number;
  /** Maximum reconnect delay. */
  maxDelayMs?: number;
  createSocket?: SocketFactory;
}

type Handler<K extends WSEventType> = (payload: WSEventMap[K]) => void;
type HandlerMap = { [K in WSEventType]?: Set<Handler<K>> };

export function buildWsUrl(
  roomCode: string,
  token: string,
  loc: Pick<Location, "protocol" | "host"> = window.location,
): string {
  const protocol = loc.protocol === "https:" ? "wss:" : "ws:";
  const code = encodeURIComponent(roomCode.trim().toUpperCase());
  return `${protocol}//${loc.host}/api/rooms/${code}/ws?token=${encodeURIComponent(token)}`;
}

export function parseWSEvent(raw: unknown): WSEvent | null {
  if (typeof raw !== "string") return null;
  let data: unknown;
  try {
    data = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof data !== "object" || data === null) return null;
  const msg = data as { type?: unknown; payload?: unknown };
  if (typeof msg.type !== "string") return null;
  if (!(WS_EVENT_TYPES as readonly string[]).includes(msg.type)) return null;
  if (typeof msg.payload !== "object" || msg.payload === null) return null;
  return msg as WSEvent;
}

/**
 * Resilient WebSocket connection with exponential backoff (unlimited retries).
 * Calling `disconnect()` closes intentionally and never reconnects.
 */
export class WebSocketManager {
  private socket: SocketLike | null = null;
  private attempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private intentionallyClosed = false;
  private status: ConnectionStatus = "closed";
  private readonly handlers: HandlerMap = {};
  private readonly baseDelayMs: number;
  private readonly maxDelayMs: number;
  private readonly createSocket: SocketFactory;
  private readonly url: string;
  private readonly options: WebSocketManagerOptions;

  constructor(url: string, options: WebSocketManagerOptions = {}) {
    this.url = url;
    this.options = options;
    this.baseDelayMs = options.baseDelayMs ?? 500;
    this.maxDelayMs = options.maxDelayMs ?? 10_000;
    this.createSocket = options.createSocket ?? ((u) => new WebSocket(u));
  }

  get connectionStatus(): ConnectionStatus {
    return this.status;
  }

  isConnected(): boolean {
    return this.status === "open";
  }

  /** Subscribe to one event type. Returns an unsubscribe function. */
  on<K extends WSEventType>(type: K, handler: Handler<K>): () => void {
    let set = this.handlers[type] as Set<Handler<K>> | undefined;
    if (!set) {
      set = new Set<Handler<K>>();
      (this.handlers as Record<K, Set<Handler<K>>>)[type] = set;
    }
    set.add(handler);
    return () => {
      set.delete(handler);
    };
  }

  connect(): void {
    this.intentionallyClosed = false;
    this.clearReconnectTimer();
    if (this.socket) return;
    this.open();
  }

  disconnect(): void {
    this.intentionallyClosed = true;
    this.clearReconnectTimer();
    const socket = this.socket;
    this.socket = null;
    if (socket) {
      this.detach(socket);
      try {
        socket.close(1000, "client disconnect");
      } catch {
        // ignore
      }
    }
    this.attempts = 0;
    this.setStatus("closed");
  }

  /** Delay before the n-th reconnect attempt (0-based). */
  nextDelay(attempt: number): number {
    return Math.min(this.maxDelayMs, this.baseDelayMs * 2 ** attempt);
  }

  private open(): void {
    this.setStatus("connecting");
    let socket: SocketLike;
    try {
      socket = this.createSocket(this.url);
    } catch {
      this.scheduleReconnect();
      return;
    }
    this.socket = socket;

    socket.onopen = () => {
      this.attempts = 0;
      this.setStatus("open");
    };
    socket.onmessage = (ev: MessageEvent) => {
      const event = parseWSEvent(ev.data);
      if (event) this.dispatch(event);
    };
    socket.onerror = () => {
      // `close` always follows `error`; reconnect is handled there.
    };
    socket.onclose = () => {
      this.detach(socket);
      if (this.socket === socket) this.socket = null;
      if (this.intentionallyClosed) return;
      this.setStatus("closed");
      this.scheduleReconnect();
    };
  }

  private dispatch(event: WSEvent): void {
    this.options.onEvent?.(event);
    const set = this.handlers[event.type] as
      | Set<(payload: WSEvent["payload"]) => void>
      | undefined;
    set?.forEach((h) => h(event.payload));
  }

  private scheduleReconnect(): void {
    if (this.intentionallyClosed) return;
    this.clearReconnectTimer();
    const delay = this.nextDelay(this.attempts);
    this.attempts++;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (!this.intentionallyClosed) this.open();
    }, delay);
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private detach(socket: SocketLike): void {
    socket.onopen = null;
    socket.onmessage = null;
    socket.onerror = null;
    socket.onclose = null;
  }

  private setStatus(status: ConnectionStatus): void {
    if (this.status === status) return;
    this.status = status;
    this.options.onStatusChange?.(status);
  }
}
