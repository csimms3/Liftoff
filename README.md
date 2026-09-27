# Liftoff - Workout Tracking Application

A full-stack workout tracking application designed to help users create, track, and manage their fitness routines.

## Features

- **User Authentication**: Register, login, session timeout (password reset is disabled until email sending is wired up)
- **Workout Management**: Create, edit, and delete workout plans (per-user)
- **Exercise Tracking**: Add exercises with sets, reps, and weights
- **Exercise Templates**: Quick-add common exercises from predefined templates
- **Workout Sessions**: Track active workout sessions and progress
- **Progress Tracking**: Monitor your fitness journey over time
- **Responsive Design**: Works seamlessly on desktop and mobile devices

## Architecture

### Backend (Go)
- **Framework**: Gin web framework
- **Database**: PostgreSQL
- **Data Access**: pgx / database/sql with repository pattern
- **Auth**: JWT (access tokens) with AuthMiddleware for protected routes

### Frontend (React + TypeScript)
- **Framework**: React 19 with TypeScript
- **Build Tool**: Vite for fast development and building
- **Styling**: CSS with responsive design principles
- **State Management**: React hooks, AuthContext for auth state

## Project Structure

```
Liftoff/
├── backend/                 # Go backend application
│   ├── auth/               # JWT auth and middleware
│   ├── database/           # Database connection and migration runner
│   ├── handlers/           # HTTP handlers (auth, admin)
│   ├── middleware/         # CORS, rate limiting
│   ├── migrations/         # Ordered SQL migrations, applied on startup
│   ├── models/             # Data models and structs
│   ├── repository/         # Data access layer
│   ├── internal/testdb/    # Test databases (per-test Postgres schemas)
│   ├── main.go             # Main application entry point
│   └── go.mod              # Go module dependencies
├── frontend/                # React frontend application
│   ├── src/
│   │   ├── components/      # React components (AuthGate, LoginPage, etc.)
│   │   ├── context/        # AuthContext
│   │   ├── api.ts          # API service and interfaces
│   │   ├── App.tsx         # Main application component
│   │   └── App.css         # Application styles
│   ├── package.json
│   └── vite.config.ts      # Vite config (proxies /api to backend)
├── scripts/
│   └── boot.sh             # Start backend + frontend
├── docs/
│   └── architecture.md     # Architecture overview
├── .github/workflows/      # CI: vet, lint, build, tests
├── scripts/dev-db.sh       # Project-local PostgreSQL for dev and tests (.pgdata/)
├── docker-compose.yml      # Alternative: PostgreSQL via Docker
└── README.md               # This file
```

## Setup & Installation

### Quick start (run everything)
```bash
# From project root - install deps first if needed
cd frontend && pnpm install && cd ..
./scripts/boot.sh
```
Starts backend (8080) and frontend (5173). Open http://localhost:5173. Press Ctrl+C to stop both.

### Prerequisites
- Go 1.24+
- Node.js 22 (see `frontend/.nvmrc`; Node 26 breaks the jsdom tests) and pnpm (`corepack enable`)
- PostgreSQL 16 binaries on PATH: `brew install postgresql@16`, then (it's keg-only)
  `export PATH="$(brew --prefix postgresql@16)/bin:$PATH"`. `scripts/dev-db.sh` runs a
  project-local server in `.pgdata/`; no other Postgres is touched

### Backend Setup
```bash
scripts/dev-db.sh start  # project-local PostgreSQL (the URL is in .env.example)
cd backend
cp .env.example .env     # then set JWT_SECRET: openssl rand -hex 32
go mod download
go run .
```
Run the backend from `backend/`: `.env` is read from the
current directory. `scripts/boot.sh` does this for you.

The backend will start on `http://localhost:8080`

### Frontend Setup
```bash
cd frontend
pnpm install
pnpm dev
```

The frontend will start on `http://localhost:5173` and proxy `/api` to the backend in development.

### Database Setup
- **`DATABASE_URL`** (required): the PostgreSQL to use. If it is unreachable, the
  server still starts, `/api/*` answers `503` (the login screen shows why), and it reconnects in
  the background. `GET /api/status` reports database availability.
- **Local development**: `scripts/dev-db.sh start` runs a project-local PostgreSQL (data in
  `.pgdata/`, port 55433, no password); `scripts/boot.sh` starts it and adds its
  `DATABASE_URL` to `backend/.env`. For a second checkout on the same machine, set
  `LIFTOFF_DEV_DB_PORT` and put `$(scripts/dev-db.sh url)` in that checkout's `backend/.env`.

Schema migrations in `backend/migrations/` are applied on startup and recorded in
`schema_migrations`.

### Admin access
There is no default admin account. To make a user an admin, set the flag by hand:
```sql
UPDATE users SET is_admin = true WHERE email = 'you@example.com';
```

### Configuration (env or `backend/.env`, see `backend/.env.example`)
- `JWT_SECRET` - **Required**, at least 32 characters (`openssl rand -hex 32`). The server refuses to start without it; `scripts/boot.sh` generates one into `backend/.env` for local dev
- `JWT_EXPIRY_MINUTES` - Session token expiry without "remember me" (default: 720, i.e. 12 hours)
- `CORS_ALLOWED_ORIGINS` - Comma-separated origins allowed cross-origin (default: none; not needed same-origin or in dev)
- `TRUSTED_PROXIES` - Comma-separated proxy IPs/CIDRs trusted for `X-Forwarded-For` (default: none)

Login is limited to 10 requests/minute and registration to 5/hour per client IP.

## API Endpoints

### Authentication (public)
- `POST /api/auth/register` - Register new user
- `POST /api/auth/login` - Login
- `GET /api/auth/me` - Get current user (requires `Authorization: Bearer <token>`)

### Workouts (require auth)
- `GET /api/workouts` - List workouts for current user
- `POST /api/workouts` - Create new workout
- `GET /api/workouts/:id` - Get specific workout
- `DELETE /api/workouts/:id` - Delete workout

### Exercises (require auth)
- `POST /api/exercises` - Add exercise to workout
- `DELETE /api/exercises/:id` - Remove exercise
- `GET /api/workouts/:id/exercises` - Get exercises for workout

### Exercise Templates (require auth)
- `GET /api/exercise-templates` - Get predefined exercise templates

### Sessions (require auth)
- `POST /api/sessions` - Start workout session
- `GET /api/sessions/active` - Get active session
- `PUT /api/sessions/:id/end` - End workout session

## Exercise Templates

The application includes 32 predefined exercise templates organized by muscle group:

- **Chest**: Barbell Bench Press, Dumbbell Bench Press, Push-ups
- **Back**: Pull-ups, Barbell Rows, Dumbbell Rows
- **Shoulders**: Overhead Press, Lateral Raises, Front Raises
- **Arms**: Bicep Curls, Tricep Dips, Hammer Curls
- **Legs**: Barbell Squats, Deadlifts, Lunges
- **Core**: Plank, Crunches, Russian Twists
- **Cardio**: Running, Cycling, Jump Rope

## Development

### Code Style
- **Go**: Follow Go formatting standards (`gofmt`)
- **TypeScript**: Use strict mode and consistent naming
- **CSS**: BEM methodology for component styling

### Testing
```bash
# Backend tests: need PostgreSQL; `make test` starts the dev database and sets
# LIFTOFF_TEST_DATABASE_URL (each test uses its own schema)
make test

# Frontend tests
cd frontend
pnpm test
```

### Building
```bash
# Backend
cd backend
go build -o liftoff

# Frontend
cd frontend
pnpm build
```

## Deployment

Not deployed yet. `go build` in `backend/` produces the API server, which needs
`JWT_SECRET` and `DATABASE_URL` (see `backend/.env.example`; its `DATABASE_URL` is the local dev database, so set a real one); `pnpm build` in `frontend/`
produces static files in `frontend/dist/`. How the two are hosted together is still to be
decided.
