export interface StoredSession {
  token: string;
  roomCode: string;
}

export const SESSION_STORAGE_KEY = "pointpoker.session";

export function normalizeRoomCode(code: string): string {
  return code.trim().toUpperCase();
}

function isStoredSession(value: unknown): value is StoredSession {
  if (typeof value !== "object" || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.token === "string" &&
    v.token.length > 0 &&
    typeof v.roomCode === "string" &&
    v.roomCode.length > 0
  );
}

export function loadSession(): StoredSession | null {
  try {
    const raw = localStorage.getItem(SESSION_STORAGE_KEY);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!isStoredSession(parsed)) return null;
    return { token: parsed.token, roomCode: normalizeRoomCode(parsed.roomCode) };
  } catch {
    return null;
  }
}

export function saveSession(session: StoredSession): void {
  const value: StoredSession = {
    token: session.token,
    roomCode: normalizeRoomCode(session.roomCode),
  };
  try {
    localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(value));
  } catch {
    // Storage unavailable (private mode / quota) — session lives in memory only.
  }
}

export function clearStoredSession(): void {
  try {
    localStorage.removeItem(SESSION_STORAGE_KEY);
    // Legacy key from the MVP.
    localStorage.removeItem("sessionToken");
  } catch {
    // ignore
  }
}

/** True when the stored session belongs to the given room. */
export function hasSessionFor(code: string): boolean {
  const s = loadSession();
  return s !== null && s.roomCode === normalizeRoomCode(code);
}
