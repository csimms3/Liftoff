# Liftoff — Frontend

React + TypeScript + Vite frontend for the Liftoff workout tracker.

## Dev server

```bash
pnpm install
pnpm dev        # starts on http://localhost:5173
```

The Vite dev server proxies `/api` requests to the backend at `localhost:8080`.
Start the backend first, or use `./scripts/boot.sh` from the repo root to run both together.

## Tests

```bash
pnpm test       # watch mode
pnpm test:run   # single run
```

## Build

```bash
pnpm build      # outputs to dist/
pnpm preview    # preview the production build locally
```
