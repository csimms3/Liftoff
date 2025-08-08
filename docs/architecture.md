# Liftoff Architecture

## Overview
Liftoff is a full-stack workout tracking application with a React frontend and Go backend.

## Tech Stack
- **Frontend**: React 19 + TypeScript + Vite
- **Backend**: Go + Gin + GraphQL + PostgreSQL/SQLite
- **Database**: PostgreSQL (production) / SQLite (development)
- **Package Manager**: pnpm

## Architecture Diagram

```
┌─────────────────┐    HTTP/GraphQL    ┌─────────────────┐
│   React App     │◄──────────────────►│   Go Backend    │
│   (localhost:5173)│                   │  (localhost:8080)│
└─────────────────┘                    └─────────────────┘
         │                                       │
         │ localStorage                           │
         │ (fallback)                            │
         ▼                                       ▼
┌─────────────────┐                    ┌─────────────────┐
│   Browser       │                    │   SQLite/PostgreSQL│
│   Storage       │                    │   Database      │
└─────────────────┘                    └─────────────────┘
```

## Data Flow
1. **Frontend** makes API calls to backend
2. **Backend** processes requests and queries database
3. **Database** stores workout data persistently
4. **Frontend** updates UI with response data

## API Endpoints
- `GET /api/workouts` - List all workouts
- `POST /api/workouts` - Create new workout
- `GET /api/workouts/:id` - Get specific workout
- `POST /api/exercises` - Add exercise to workout
- `GET /api/workouts/:id/exercises` - Get exercises for workout
- `POST /api/sessions` - Start workout session
- `GET /api/sessions/active` - Get active session
- `PUT /api/sessions/:id/end` - End session

## Database Schema
- `workouts` - Workout definitions
- `exercises` - Exercise definitions within workouts
- `workout_sessions` - Active workout sessions
- `session_exercises` - Exercises in a session
- `exercise_sets` - Individual sets performed

## Development Workflow
1. Start backend: `cd backend && go run main.go`
2. Start frontend: `cd frontend && pnpm dev`
3. Access app: http://localhost:5173

## Current Status
- ✅ **Stage 1**: API service created, frontend connected to backend
- ✅ **Stage 2**: Backend with SQLite support, basic API working
- ✅ **Stage 3**: Frontend polish and error handling
- ✅ **Stage 4**: Basic testing and backend fixes
- 🚀 **Ready for production**: Core functionality complete

## Next Steps
- Add more comprehensive tests
- Set up production deployment (Vercel/Netlify)
- Add user authentication
- Implement GraphQL resolvers
- Add exercise library and progress analytics
