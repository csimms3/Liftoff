# Liftoff Architecture

## Overview
Liftoff is a full-stack workout tracking application with a React frontend and Go backend. All workout data is isolated per user via JWT authentication.

## Tech Stack
- **Frontend**: React 18 + TypeScript + Vite
- **Backend**: Go + Gin (REST) + pgx / SQLite
- **Database**: PostgreSQL (production) / SQLite (development)
- **Auth**: JWT (access tokens) with Bearer scheme
- **Package Manager**: pnpm

## Architecture Diagram

```
┌─────────────────┐    REST /api      ┌─────────────────┐
│   React App     │◄──────────────────►│   Go Backend    │
│   (localhost:5173)│  Bearer JWT       │  (localhost:8080)│
└─────────────────┘                    └─────────────────┘
         │                                       │
         │ localStorage (auth)                   │
         ▼                                       ▼
┌─────────────────┐                    ┌─────────────────┐
│   AuthContext   │                    │   SQLite/PostgreSQL│
│   + JWT token   │                    │   (user_id isolation)│
└─────────────────┘                    └─────────────────┘
```

## Data Flow
1. **Frontend** authenticates (register/login) and stores JWT in localStorage
2. **API calls** include `Authorization: Bearer <token>` header
3. **Backend** validates JWT via AuthMiddleware and scopes queries by `user_id`
4. **Database** stores user-scoped workout data

## API Endpoints
- `POST /api/auth/register` - Register
- `POST /api/auth/login` - Login
- `POST /api/auth/forgot-password` - Request reset
- `POST /api/auth/reset-password` - Reset password
- `GET /api/auth/me` - Current user (protected)
- `GET /api/workouts` - List workouts (protected)
- `POST /api/workouts` - Create workout (protected)
- `GET /api/workouts/:id` - Get workout (protected)
- `POST /api/exercises` - Add exercise (protected)
- `GET /api/workouts/:id/exercises` - Get exercises (protected)
- `POST /api/sessions` - Start session (protected)
- `GET /api/sessions/active` - Active session (protected)
- `PUT /api/sessions/:id/end` - End session (protected)

## Database Schema
- `users` - User accounts (id, email, password_hash)
- `workouts` - Workout definitions (user_id for isolation)
- `exercises` - Exercise definitions within workouts
- `workout_sessions` - Active workout sessions (user_id)
- `session_exercises` - Exercises in a session
- `exercise_sets` - Individual sets performed
- `dino_scores` - Game scores (user_id)

## Development Workflow
1. Start backend: `cd backend && go run main.go`
2. Start frontend: `cd frontend && pnpm dev`
3. Access app: http://localhost:5173 (Vite proxies /api to backend)

## Current Status
- User auth (register, login, forgot/reset password, session timeout)
- Data isolation by user_id
- Core workout, exercise, session, progress features
- Exercise templates and progress analytics
- Ready for production (set JWT_SECRET and deploy)

## Next Steps
- Add more comprehensive tests
- Set up production deployment
