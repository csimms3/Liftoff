# Liftoff

A fullstack workout tracking application that empowers users to create custom workouts, track their lifting sessions, and monitor their fitness progress.

## Features

- **Workout Creation**: Design custom workouts with exercises, sets, and reps
- **Session Tracking**: Start workout sessions and log your lifts in real-time
- **Progress Monitoring**: Track your strength gains and workout history
- **Exercise Management**: Add and remove exercises from workouts

## Tech Stack

- **Frontend**: React 19 + TypeScript + Vite
- **Backend**: Go + Gin + REST API
- **Database**: PostgreSQL (production) / SQLite (development)

## Getting Started

### Prerequisites
- Go 1.21+
- Node.js 18+
- pnpm (recommended) or npm

### Backend Setup
1. Navigate to the backend directory:
   ```bash
   cd backend
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Start the server:
   ```bash
   go run main.go
   ```
   
   The server will start on `http://localhost:8080`

### Frontend Setup
1. Navigate to the frontend directory:
   ```bash
   cd frontend
   ```

2. Install dependencies:
   ```bash
   pnpm install
   ```

3. Start the development server:
   ```bash
   pnpm dev
   ```
   
   The app will be available at `http://localhost:5173`

### Database Setup
The application automatically uses SQLite for development. For PostgreSQL:

1. Start PostgreSQL with Docker:
   ```bash
   make db-up
   ```

2. Run migrations:
   ```bash
   make migrate
   ```

## API Endpoints

- `GET /api/workouts` - List all workouts
- `POST /api/workouts` - Create new workout
- `GET /api/workouts/:id` - Get specific workout
- `DELETE /api/workouts/:id` - Delete workout
- `POST /api/exercises` - Add exercise to workout
- `DELETE /api/exercises/:id` - Delete exercise
- `GET /api/workouts/:id/exercises` - Get exercises for workout
- `POST /api/sessions` - Start workout session
- `GET /api/sessions/active` - Get active session
- `PUT /api/sessions/:id/end` - End session
- `PUT /api/exercise-sets/:id/complete` - Complete exercise set
- `GET /api/progress` - Get progress data

## Development

- Backend: `make dev` or `go run main.go`
- Frontend: `cd frontend && pnpm dev`
- Tests: `make test`
- Database: `make db-up` for PostgreSQL or SQLite fallback

---

*More features and documentation to come as the project evolves.*