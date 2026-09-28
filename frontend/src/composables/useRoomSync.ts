import { onBeforeUnmount } from "vue";
import { buildWsUrl, WebSocketManager } from "../api/websocket";
import type { WSEvent } from "../api/types";
import { useAppStore } from "../stores/app";

export const REFRESH_DEBOUNCE_MS = 150;
export const FALLBACK_POLL_MS = 5000;

/**
 * Keeps the store in sync with the server:
 * - `presence` events update the online set directly;
 * - every other event triggers a debounced REST refetch;
 * - while the socket is down, poll every 5s as a fallback.
 */
export function useRoomSync() {
  const store = useAppStore();
  let manager: WebSocketManager | null = null;
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  let pollTimer: ReturnType<typeof setInterval> | null = null;
  let hasBeenOpen = false;

  function refreshNow(): void {
    store.refreshAll().catch(() => {
      // Background refresh; next event/poll will retry.
    });
  }

  function scheduleRefresh(): void {
    if (debounceTimer !== null) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      debounceTimer = null;
      refreshNow();
    }, REFRESH_DEBOUNCE_MS);
  }

  function startPolling(): void {
    if (pollTimer !== null) return;
    pollTimer = setInterval(refreshNow, FALLBACK_POLL_MS);
  }

  function stopPolling(): void {
    if (pollTimer !== null) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  }

  function handleEvent(event: WSEvent): void {
    if (event.type === "presence") {
      store.setOnline(event.payload.online);
    } else {
      scheduleRefresh();
    }
  }

  function start(code: string, token: string): void {
    stop();
    manager = new WebSocketManager(buildWsUrl(code, token), {
      onEvent: handleEvent,
      onStatusChange: (status) => {
        store.setWsConnected(status === "open");
        if (status === "open") {
          stopPolling();
          // Catch up on anything missed while reconnecting.
          if (hasBeenOpen) scheduleRefresh();
          hasBeenOpen = true;
        } else if (status === "closed") {
          startPolling();
        }
      },
    });
    manager.connect();
  }

  function stop(): void {
    manager?.disconnect();
    manager = null;
    hasBeenOpen = false;
    stopPolling();
    if (debounceTimer !== null) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
    store.setWsConnected(false);
  }

  onBeforeUnmount(stop);

  return { start, stop, scheduleRefresh };
}
