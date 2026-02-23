# Liftoff - Workout Tracking Application

A full-stack workout tracking application built with Go backend and React frontend, designed to help users create, track, and manage their fitness routines.

## Features

- **User Authentication**: Register, login, forgot password, reset password, session timeout
- **Workout Management**: Create, edit, and delete workout plans (per-user)
- **Exercise Tracking**: Add exercises with sets, reps, and weights
- **Exercise Templates**: Quick-add common exercises from predefined templates
- **Workout Sessions**: Track active workout sessions and progress
- **Progress Tracking**: Monitor your fitness journey over time
- **Responsive Design**: Works seamlessly on desktop and mobile devices

## Architecture

### Backend (Go)
- **Framework**: Gin web framework
- **Database**: PostgreSQL (primary) with SQLite fallback
- **Data Access**: pgx / database/sql with repository pattern
- **Auth**: JWT (access tokens) with AuthMiddleware for protected routes

### Frontend (React + TypeScript)
- **Framework**: React 18 with TypeScript
- **Build Tool**: Vite for fast development and building
- **Styling**: CSS with responsive design principles
- **State Management**: React hooks, AuthContext for auth state

## Project Structure

```
Liftoff/
├── backend/                 # Go backend application
│   ├── auth/               # JWT auth and middleware
│   ├── database/           # Database connection and configuration
│   ├── handlers/            # HTTP handlers (auth, etc.)
│   ├── models/             # Data models and structs
│   ├── repository/         # Data access layer
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
├── docs/
│   └── architecture.md     # Architecture overview
├── docker-compose.yml      # Docker setup for PostgreSQL
└── README.md               # This file
```

## Setup & Installation

### Prerequisites
- Go 1.21+ 
- Node.js 18+ and pnpm
- PostgreSQL (optional, SQLite will be used as fallback)

### Backend Setup
```bash
cd backend
go mod download
go run main.go
```

The backend will start on `http://localhost:8080`

### Frontend Setup
```bash
cd frontend
pnpm install
pnpm dev
```

The frontend will start on `http://localhost:5173` and proxy `/api` to the backend in development.

### Database Setup
The application automatically detects and connects to:
1. PostgreSQL (if available)
2. SQLite (fallback, creates `liftoff.db` file)

### Auth (optional env)
- `JWT_SECRET` - Secret for signing tokens (default: dev secret)
- `JWT_EXPIRY_MINUTES` - Session token expiry (default: 15)

## API Endpoints

### Authentication (public)
- `POST /api/auth/register` - Register new user
- `POST /api/auth/login` - Login
- `POST /api/auth/forgot-password` - Request password reset email
- `POST /api/auth/reset-password` - Reset password with token
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
# Backend tests
cd backend
go test ./...

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

### Backend
The Go backend can be deployed as a single binary:
```bash
go build -o liftoff
./liftoff
```

### Frontend
Build the frontend and serve static files:
```bash
pnpm build
# Serve dist/ directory with any static file server
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is open source and available under the [MIT License](LICENSE).

## Support

If you encounter any issues or have questions:
1. Check the existing issues
2. Create a new issue with detailed information
3. Include steps to reproduce the problem

---

Built with for fitness enthusiasts everywhere.