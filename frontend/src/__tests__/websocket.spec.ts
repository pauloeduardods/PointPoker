import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { buildWsUrl, parseWSEvent, WebSocketManager, type ConnectionStatus } from "../api/websocket";
import type { WSEvent } from "../api/types";
import { FakeWebSocket } from "./helpers/fakeWebSocket";

function setup(opts: { base?: number; max?: number } = {}) {
  const events: WSEvent[] = [];
  const statuses: ConnectionStatus[] = [];
  const mgr = new WebSocketManager("ws://test/ws", {
    baseDelayMs: opts.base ?? 100,
    maxDelayMs: opts.max ?? 1000,
    createSocket: (url) => new FakeWebSocket(url),
    onEvent: (e) => events.push(e),
    onStatusChange: (s) => statuses.push(s),
  });
  return { mgr, events, statuses };
}

describe("WebSocketManager", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    FakeWebSocket.reset();
  });
  afterEach(() => vi.useRealTimers());

  it("builds the WS url from window.location", () => {
    expect(buildWsUrl("abc", "t k", { protocol: "https:", host: "poker.example.com" })).toBe(
      "wss://poker.example.com/api/rooms/ABC/ws?token=t%20k",
    );
    expect(buildWsUrl("ABC", "t", { protocol: "http:", host: "localhost:5173" })).toBe(
      "ws://localhost:5173/api/rooms/ABC/ws?token=t",
    );
  });

  it("reports status transitions", () => {
    const { mgr, statuses } = setup();
    mgr.connect();
    FakeWebSocket.latest().serverOpen();
    expect(statuses).toEqual(["connecting", "open"]);
    expect(mgr.isConnected()).toBe(true);
  });

  it("reconnects with exponential backoff capped at max, without giving up", () => {
    const { mgr } = setup({ base: 100, max: 1000 });
    mgr.connect();
    expect(FakeWebSocket.instances).toHaveLength(1);

    const expectedDelays = [100, 200, 400, 800, 1000, 1000, 1000, 1000];
    for (const [i, delay] of expectedDelays.entries()) {
      FakeWebSocket.latest().serverClose();
      vi.advanceTimersByTime(delay - 1);
      expect(FakeWebSocket.instances).toHaveLength(i + 1);
      vi.advanceTimersByTime(1);
      expect(FakeWebSocket.instances).toHaveLength(i + 2);
    }
  });

  it("resets backoff after a successful connection", () => {
    const { mgr } = setup({ base: 100 });
    mgr.connect();
    FakeWebSocket.latest().serverClose();
    vi.advanceTimersByTime(100);
    FakeWebSocket.latest().serverClose();
    vi.advanceTimersByTime(200);
    FakeWebSocket.latest().serverOpen();
    FakeWebSocket.latest().serverClose();
    vi.advanceTimersByTime(100);
    expect(FakeWebSocket.instances).toHaveLength(4);
  });

  it("does not reconnect after an intentional disconnect", () => {
    const { mgr, statuses } = setup();
    mgr.connect();
    const ws = FakeWebSocket.latest();
    ws.serverOpen();
    mgr.disconnect();
    expect(ws.closeCalls).toBe(1);
    ws.serverClose(); // late close event from browser
    vi.advanceTimersByTime(60_000);
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(statuses.at(-1)).toBe("closed");
  });

  it("cancels a pending reconnect on disconnect", () => {
    const { mgr } = setup();
    mgr.connect();
    FakeWebSocket.latest().serverClose();
    mgr.disconnect();
    vi.advanceTimersByTime(60_000);
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("dispatches typed events to onEvent and per-type handlers", () => {
    const { mgr, events } = setup();
    const presence = vi.fn();
    const off = mgr.on("presence", presence);
    mgr.connect();
    const ws = FakeWebSocket.latest();
    ws.serverOpen();

    ws.emit("presence", { online: ["a", "b"] });
    ws.emit("vote_cast", { participant_id: "a" });
    expect(presence).toHaveBeenCalledWith({ online: ["a", "b"] });
    expect(events.map((e) => e.type)).toEqual(["presence", "vote_cast"]);

    off();
    ws.emit("presence", { online: [] });
    expect(presence).toHaveBeenCalledTimes(1);
  });

  it("ignores malformed and unknown messages", () => {
    const { mgr, events } = setup();
    mgr.connect();
    const ws = FakeWebSocket.latest();
    ws.sendRaw("not json");
    ws.sendRaw(JSON.stringify({ type: "mystery", payload: {} }));
    ws.sendRaw(JSON.stringify({ type: "presence" }));
    expect(events).toHaveLength(0);
    expect(parseWSEvent(JSON.stringify({ type: "round_reset", payload: { round: {} } }))?.type).toBe(
      "round_reset",
    );
  });
});
