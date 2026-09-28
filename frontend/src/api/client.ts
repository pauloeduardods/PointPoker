import axios from "axios";
import { loadSession } from "../utils/session";
import type { ApiErrorBody } from "./types";

/** All API calls go through the same origin; dev server / nginx proxy `/api`. */
export const API_BASE_URL = "/api";

const client = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
});

client.interceptors.request.use((config) => {
  const token = loadSession()?.token;
  if (token) {
    config.headers.set("X-Session-Token", token);
  }
  return config;
});

function isApiErrorBody(data: unknown): data is ApiErrorBody {
  return (
    typeof data === "object" &&
    data !== null &&
    typeof (data as Record<string, unknown>).error === "string"
  );
}

/** HTTP status of a failed request, or null if there was no response. */
export function errorStatus(err: unknown): number | null {
  if (axios.isAxiosError(err)) return err.response?.status ?? null;
  return null;
}

const STATUS_MESSAGES: Record<number, string> = {
  400: "Please check the information you entered.",
  401: "Your session has expired. Please join the room again.",
  403: "You don't have permission to do that.",
  404: "We couldn't find that room.",
  409: "That action isn't possible right now — the round has changed.",
};

/** Turn any thrown value into a message that can be shown to users. */
export function friendlyError(err: unknown, fallback = "Something went wrong. Please try again."): string {
  if (axios.isAxiosError(err)) {
    const res = err.response;
    if (!res) {
      return "Can't reach the server. Check your connection and try again.";
    }
    if (isApiErrorBody(res.data) && res.data.error.trim() !== "") {
      const msg = res.data.error.trim();
      return msg.charAt(0).toUpperCase() + msg.slice(1);
    }
    if (res.status >= 500) return "The server had a problem. Please try again in a moment.";
    return STATUS_MESSAGES[res.status] ?? fallback;
  }
  return fallback;
}

export default client;
