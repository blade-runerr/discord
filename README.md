# Mini Discord (Go + WebRTC + TURN)

## Local run (without Docker)

```bash
go run ./cmd/server
```

Open: `http://localhost:8000`

## Docker run (app + TURN)

1. Create env file:

```bash
cp .env.example .env
```

2. Start stack:

```bash
docker compose up --build
```

3. Open app:

`http://localhost:8000`

## TURN notes

- TURN service runs as `coturn` in `docker-compose.yml`.
- App reads TURN config from env vars:
  - `TURN_HOST`
  - `TURN_PORT`
  - `TURN_USERNAME`
  - `TURN_PASSWORD`
- Browser gets ICE servers from `GET /api/webrtc-config`.

For production, set `TURN_HOST` to your public domain/IP and use strong credentials.
