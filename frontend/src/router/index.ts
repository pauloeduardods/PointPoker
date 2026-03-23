import { createRouter, createWebHistory } from "vue-router";
import { useAppStore } from "../stores/app";
import HomeView from "../views/HomeView.vue";
import RoomView from "../views/RoomView.vue";

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/",
      name: "home",
      component: HomeView,
    },
    {
      path: "/room/:code",
      name: "room",
      component: RoomView,
      beforeEnter: (_to, _from, next) => {
        const store = useAppStore();
        if (store.sessionToken) {
          next();
        } else {
          next("/");
        }
      },
    },
  ],
});

export default router;
