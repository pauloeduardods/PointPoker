# Troubleshooting

**`mise run dev` falha ao subir o banco / "port is already allocated"**
Outro Postgres está usando a porta. O padrão do projeto é `DB_PORT=5433`; ajuste no `.env` se ela também estiver ocupada.

**Frontend abre, mas as chamadas dão 502/erro de proxy**
O backend não está rodando ou está em outra porta. O Vite faz proxy para `http://localhost:$API_PORT` (padrão 8080),
então rode o frontend pelo mise (`mise run frontend:dev`), que carrega o `.env`, ou exporte `API_PORT`.

**"Disconnected" no topo da sala**
O WebSocket caiu. O cliente reconecta sozinho (backoff de até 10s) e, enquanto isso, atualiza por polling a cada 5s.
Confira os logs do backend.

**403 "origin not allowed" chamando a API de outro domínio**
Chamadas da mesma origem (via Vite ou nginx) nunca são bloqueadas. Para chamar de outra origem, adicione-a em `CORS_ORIGINS`.

**Voltei para a tela inicial ao abrir a sala**
A sessão é guardada por sala no `localStorage`. Se o token for de outra sala ou tiver expirado (você saiu, ou o banco
foi resetado), a página pede para entrar de novo, com o código já preenchido.

**Banco em estado estranho**
`mise run db:reset` apaga o volume e recria o banco (as migrations rodam no próximo start do backend).
