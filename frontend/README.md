# Point Poker — frontend

Vue 3 + TypeScript + Vite + Pinia + Tailwind. Veja o [README da raiz](../README.md) para rodar o projeto inteiro.

```bash
npm run dev        # http://localhost:5173 (proxy de /api para localhost:$API_PORT, padrão 8080)
npm test           # Vitest
npm run lint       # vue-tsc
npm run build
```

- `src/api/`: cliente REST (axios, `/api` relativo), tipos e `WebSocketManager` (reconexão com backoff).
- `src/composables/useRoomSync.ts`: presença via WebSocket, refetch com debounce e polling só quando desconectado.
- `src/stores/app.ts`: estado da sala. `src/utils/`: estatísticas, sessão e clipboard (funções puras e testadas).
