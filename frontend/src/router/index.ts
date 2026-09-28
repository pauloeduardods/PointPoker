import {
  createRouter,
  createWebHistory,
  type RouteLocationNormalized,
  type RouteLocationRaw,
  type RouterHistory,
  type RouteRecordRaw,
} from "vue-router";
import HomeView from "../views/HomeView.vue";
import { hasSessionFor, normalizeRoomCode } from "../utils/session";

/** Visiting a room without a session for it sends you to the Join form, pre-filled. */
export function roomGuard(to: RouteLocationNormalized): true | RouteLocationRaw {
  const raw = to.params.code;
  const code = normalizeRoomCode(Array.isArray(raw) ? (raw[0] ?? "") : raw);
  if (!code) return { name: "home" };
  if (!hasSessionFor(code)) {
    return { name: "home", query: { join: code } };
  }
  return true;
}

export const routes: RouteRecordRaw[] = [
  { path: "/", name: "home", component: HomeView },
  {
    path: "/room/:code",
    name: "room",
    component: () => import("../views/RoomView.vue"),
    beforeEnter: roomGuard,
  },
  { path: "/:pathMatch(.*)*", redirect: { name: "home" } },
];

export function createAppRouter(history: RouterHistory = createWebHistory(import.meta.env.BASE_URL)) {
  return createRouter({ history, routes });
}

const router = createAppRouter();

export default router;
