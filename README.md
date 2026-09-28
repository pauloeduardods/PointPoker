# Point Poker

Planning poker em tempo real: crie uma sala, compartilhe o link, votem juntos e revelem as cartas.

- **Backend:** Go + Gin + PostgreSQL + WebSocket (`backend/`)
- **Frontend:** Vue 3 + Vite + Pinia + Tailwind (`frontend/`)
- **E2E:** Playwright (`e2e/`)

## Rodando

Pré-requisitos: [mise](https://mise.jdx.dev) e Docker.

```bash
mise run setup   # cria .env, instala ferramentas/deps, sobe o Postgres
mise run dev     # Postgres + backend (:8080) + frontend (:5173)
```

Abra http://localhost:5173. O Vite faz proxy de `/api` (REST e WebSocket) para o backend,
então não há URL de API para configurar. As migrations rodam sozinhas quando o backend sobe.

Stack completa em Docker (nginx + backend + db): `mise run up` → http://localhost:8000.

## Testes

| Comando | O que roda |
|---|---|
| `mise run test` | testes unitários do backend (`go test -race`) e do frontend (Vitest) |
| `mise run backend:test-integration` | backend + testes de repositório em um Postgres descartável (porta 55432) |
| `mise run e2e` | host e convidado em dois navegadores, fluxo completo de votação |
| `mise run lint` | `go vet` + `vue-tsc` |

## Configuração (`.env`)

| Variável | Padrão | |
|---|---|---|
| `DB_PORT` | `5433` | porta do Postgres no host (5433 para não colidir com outro Postgres na 5432) |
| `API_PORT` | `8080` | porta do backend; o proxy do Vite lê a mesma variável |
| `CORS_ORIGINS` | `http://localhost:5173,http://localhost:3000` | só importa para chamadas cross-origin; `*` libera tudo |
| `FRONTEND_PORT` | `8000` | porta do nginx em `mise run up` |

## API

Todas as rotas ficam em `/api`. A sessão vai no header `X-Session-Token` (no WebSocket, em `?token=`).

| Método | Rota | Quem |
|---|---|---|
| GET | `/health` | — |
| POST | `/rooms` | — (cria a sala; quem cria vira host) |
| GET | `/rooms/:code` | — (participantes com flag `online`) |
| POST | `/rooms/:code/join` | — |
| GET | `/rooms/:code/me` | participante (restaura a sessão após reload) |
| DELETE | `/rooms/:code/participants/me` | participante (sair; o host é transferido) |
| POST | `/rooms/:code/rounds` | host (nova história) |
| GET | `/rooms/:code/rounds/current` | — (valores ocultos até revelar, exceto o seu próprio voto) |
| POST | `/rooms/:code/rounds/:id/vote` | participante |
| POST | `/rooms/:code/rounds/:id/reveal` | host |
| POST | `/rooms/:code/rounds/:id/reset` | host (revotar) |
| GET | `/rooms/:code/ws` | participante |

Eventos do WebSocket: `presence`, `participant_joined`, `participant_left`, `round_started`,
`vote_cast`, `votes_revealed`, `round_reset`. Há uma coleção do Postman em `postman_collection.json`.
