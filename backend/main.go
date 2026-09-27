package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"liftoff/backend/auth"
	"liftoff/backend/database"
	"liftoff/backend/handlers"
	"liftoff/backend/middleware"
	"liftoff/backend/models"
	"liftoff/backend/repository"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// serverError logs err and sends a generic message, so SQL and driver details
// never reach the client.
func serverError(c *gin.Context, msg string, err error) {
	log.Printf("%s %s: %v", c.Request.Method, c.FullPath(), err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
}

// notFoundOr answers 404 with notFoundMsg for repository.ErrNotFound (missing or
// not the caller's), and a generic 500 for anything else.
func notFoundOr(c *gin.Context, err error, notFoundMsg, serverMsg string) {
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": notFoundMsg})
		return
	}
	serverError(c, serverMsg, err)
}

// badRequest answers a request body that failed to bind or validate.
func badRequest(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
}

// Liftoff API Server
// A workout tracking application with Go backend and React frontend
//
// Features:
// - Workout management (create, read, update, delete)
// - Exercise tracking with sets, reps, and weights
// - Workout sessions and progress tracking
// - Exercise templates for quick workout building

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
	if err := auth.CheckConfig(); err != nil {
		log.Fatalf("Refusing to start: %v (generate one with: openssl rand -hex 32)", err)
	}

	// Initialize database connection. Only configuration errors are fatal; an
	// unreachable Postgres leaves the server up, answering 503 until it is back.
	db, err := database.NewDatabase()
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize repositories for data access
	workoutRepo := repository.NewWorkoutRepository(db.GetPool())
	routineRepo := repository.NewRoutineRepository(db.GetPool(), workoutRepo)
	sessionRepo := repository.NewSessionRepository(db.GetPool())
	userRepo := repository.NewUserRepository(db.GetPool())
	adminRepo := repository.NewAdminRepository(db.GetPool())
	authHandler := handlers.NewAuthHandler(userRepo)
	adminHandler := handlers.NewAdminHandler(userRepo, adminRepo)

	// Setup Gin router with default middleware (Logger and Recovery)
	r := gin.Default()

	// Only trust X-Forwarded-For from these proxies (comma-separated IPs/CIDRs).
	// Unset means trust none, so ClientIP is the TCP peer and can't be spoofed.
	if err := r.SetTrustedProxies(middleware.SplitList(os.Getenv("TRUSTED_PROXIES"))); err != nil {
		log.Fatalf("Invalid TRUSTED_PROXIES: %v", err)
	}

	// Cross-origin access only for listed origins; none needed when the SPA is same-origin.
	r.Use(middleware.CORS(middleware.SplitList(os.Getenv("CORS_ALLOWED_ORIGINS"))))

	loginLimit := middleware.NewRateLimiter(10, time.Minute).Middleware()
	registerLimit := middleware.NewRateLimiter(5, time.Hour).Middleware()

	// Database status for the login screen; answers even while the DB is down.
	r.GET("/api/status", func(c *gin.Context) {
		if !db.Ready() {
			c.Header("Retry-After", "30")
			c.JSON(http.StatusServiceUnavailable, gin.H{"database": "unavailable", "error": middleware.DatabaseUnavailable})
			return
		}
		c.JSON(http.StatusOK, gin.H{"database": "ok"})
	})

	// API routes group - all endpoints under /api
	api := r.Group("/api")
	api.Use(middleware.RequireReady(db.Ready))
	{
		// Auth routes (no middleware required for login/register)
		api.POST("/auth/login", loginLimit, authHandler.Login)
		api.POST("/auth/register", registerLimit, authHandler.Register)
		// TODO(email): register /auth/forgot-password (behind its own rate limiter, e.g.
		// 3/hour) and /auth/reset-password once an email provider sends reset links.
		// Until then the handlers exist but are unreachable, and the UI hides the link.
		api.GET("/auth/me", auth.AuthMiddleware(), authHandler.Me)

		// Admin routes (auth + admin role required)
		adminAPI := api.Group("/admin")
		adminAPI.Use(auth.AuthMiddleware(), auth.AdminMiddleware(userRepo.IsAdmin))
		{
			adminAPI.GET("/users", adminHandler.ListUsers)
			adminAPI.GET("/stats", adminHandler.GetStats)
		}
	}
	authAPI := api.Group("")
	authAPI.Use(auth.AuthMiddleware())
	{
		userID := func(c *gin.Context) string { return auth.GetUserID(c) }
		// Workout management endpoints
		authAPI.GET("/workouts", func(c *gin.Context) {
			workouts, err := workoutRepo.GetWorkouts(c.Request.Context(), userID(c))
			if err != nil {
				log.Printf("Error fetching workouts: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch workouts"})
				return
			}
			if workouts == nil {
				workouts = []*models.Workout{}
			}
			c.JSON(http.StatusOK, workouts)
		})

		authAPI.POST("/workouts", func(c *gin.Context) {
			var input struct {
				Name string `json:"name" binding:"required"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Workout name is required"})
				return
			}
			workout, err := workoutRepo.CreateWorkout(c.Request.Context(), userID(c), input.Name)
			if err != nil {
				log.Printf("Error creating workout: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workout"})
				return
			}
			c.JSON(http.StatusCreated, workout)
		})

		authAPI.GET("/workouts/:id", func(c *gin.Context) {
			workout, err := workoutRepo.GetWorkout(c.Request.Context(), userID(c), c.Param("id"))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Workout not found"})
				return
			}
			c.JSON(http.StatusOK, workout)
		})

		authAPI.DELETE("/workouts/:id", func(c *gin.Context) {
			err := workoutRepo.DeleteWorkout(c.Request.Context(), userID(c), c.Param("id"))
			if err != nil {
				log.Printf("Error deleting workout: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workout"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Workout deleted successfully"})
		})

		// Routine management endpoints
		authAPI.GET("/routines", func(c *gin.Context) {
			routines, err := routineRepo.GetRoutines(c.Request.Context(), userID(c))
			if err != nil {
				log.Printf("Error fetching routines: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch routines"})
				return
			}
			if routines == nil {
				routines = []*models.Routine{}
			}
			c.JSON(http.StatusOK, routines)
		})

		authAPI.POST("/routines", func(c *gin.Context) {
			var input struct {
				Name        string   `json:"name" binding:"required"`
				Description string   `json:"description"`
				WorkoutIDs  []string `json:"workout_ids"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Routine name is required"})
				return
			}
			routine, err := routineRepo.CreateRoutine(c.Request.Context(), userID(c), input.Name, input.Description, input.WorkoutIDs)
			if errors.Is(err, repository.ErrNotFound) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "One or more workouts were not found"})
				return
			}
			if err != nil {
				serverError(c, "Failed to create routine", err)
				return
			}
			c.JSON(http.StatusCreated, routine)
		})

		authAPI.GET("/routines/:id", func(c *gin.Context) {
			routine, err := routineRepo.GetRoutine(c.Request.Context(), userID(c), c.Param("id"))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Routine not found"})
				return
			}
			c.JSON(http.StatusOK, routine)
		})

		authAPI.PUT("/routines/:id", func(c *gin.Context) {
			var input struct {
				Name        string   `json:"name"`
				Description string   `json:"description"`
				WorkoutIDs  []string `json:"workout_ids"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
				return
			}
			routine, err := routineRepo.GetRoutine(c.Request.Context(), userID(c), c.Param("id"))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Routine not found"})
				return
			}
			name, desc := routine.Name, routine.Description
			if input.Name != "" {
				name = input.Name
			}
			if input.Description != "" {
				desc = input.Description
			}
			err = routineRepo.UpdateRoutine(c.Request.Context(), userID(c), routine.ID, name, desc, input.WorkoutIDs)
			if errors.Is(err, repository.ErrNotFound) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "One or more workouts were not found"})
				return
			}
			if err != nil {
				serverError(c, "Failed to update routine", err)
				return
			}
			routine, err = routineRepo.GetRoutine(c.Request.Context(), userID(c), routine.ID)
			if err != nil {
				serverError(c, "Failed to update routine", err)
				return
			}
			c.JSON(http.StatusOK, routine)
		})

		authAPI.DELETE("/routines/:id", func(c *gin.Context) {
			err := routineRepo.DeleteRoutine(c.Request.Context(), userID(c), c.Param("id"))
			if err != nil {
				log.Printf("Error deleting routine: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete routine"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Routine deleted successfully"})
		})

		authAPI.POST("/routine-templates/:templateId/create", func(c *gin.Context) {
			var input struct {
				Name string `json:"name"`
			}
			_ = c.ShouldBindJSON(&input)
			routine, err := routineRepo.CreateFromTemplate(c.Request.Context(), userID(c), c.Param("templateId"), input.Name)
			if errors.Is(err, repository.ErrTemplateNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Routine template not found"})
				return
			}
			if err != nil {
				serverError(c, "Failed to create routine from template", err)
				return
			}
			c.JSON(http.StatusCreated, routine)
		})

		// Workout template routes
		api.GET("/workout-templates", func(c *gin.Context) {
			templates, err := workoutRepo.GetWorkoutTemplates(c.Request.Context())
			if err != nil {
				serverError(c, "Failed to fetch workout templates", err)
				return
			}
			c.JSON(http.StatusOK, templates)
		})

		api.GET("/exercise-templates", func(c *gin.Context) {
			templates, err := workoutRepo.GetExerciseTemplates(c.Request.Context())
			if err != nil {
				serverError(c, "Failed to fetch exercise templates", err)
				return
			}
			c.JSON(http.StatusOK, templates)
		})

		api.GET("/routine-templates", func(c *gin.Context) {
			templates := routineRepo.GetRoutineTemplates()
			list := make([]gin.H, len(templates))
			for i, t := range templates {
				list[i] = gin.H{"id": t.ID, "name": t.Name, "description": t.Description, "workout_count": len(t.Workouts)}
			}
			c.JSON(http.StatusOK, list)
		})

		authAPI.POST("/workout-templates/:id/create", func(c *gin.Context) {
			var req struct {
				Name string `json:"name"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				badRequest(c)
				return
			}
			workout, err := workoutRepo.CreateWorkoutFromTemplate(c.Request.Context(), userID(c), c.Param("id"), req.Name)
			if errors.Is(err, repository.ErrTemplateNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Workout template not found"})
				return
			}
			if err != nil {
				serverError(c, "Failed to create workout from template", err)
				return
			}
			c.JSON(http.StatusCreated, workout)
		})

		// Exercise routes
		authAPI.POST("/exercises", func(c *gin.Context) {
			var input struct {
				Name      string  `json:"name" binding:"required"`
				Sets      int     `json:"sets" binding:"required"`
				Reps      int     `json:"reps" binding:"required"`
				Weight    float64 `json:"weight"`
				WorkoutID string  `json:"workout_id" binding:"required"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				badRequest(c)
				return
			}

			exercise := &models.Exercise{
				Name:      input.Name,
				Sets:      input.Sets,
				Reps:      input.Reps,
				Weight:    input.Weight,
				WorkoutID: input.WorkoutID,
			}

			err := workoutRepo.CreateExercise(c.Request.Context(), userID(c), exercise)
			if err != nil {
				serverError(c, "Failed to add exercise", err)
				return
			}
			c.JSON(http.StatusCreated, exercise)
		})

		authAPI.DELETE("/exercises/:id", func(c *gin.Context) {
			err := workoutRepo.DeleteExercise(c.Request.Context(), userID(c), c.Param("id"))
			if err != nil {
				serverError(c, "Failed to delete exercise", err)
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Exercise deleted"})
		})

		authAPI.GET("/workouts/:id/exercises", func(c *gin.Context) {
			_, err := workoutRepo.GetWorkout(c.Request.Context(), userID(c), c.Param("id"))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Workout not found"})
				return
			}
			exercises, err := workoutRepo.GetExercisesByWorkout(c.Request.Context(), c.Param("id"))
			if err != nil {
				serverError(c, "Failed to fetch exercises", err)
				return
			}
			c.JSON(http.StatusOK, exercises)
		})

		// Session routes
		authAPI.POST("/sessions", func(c *gin.Context) {
			var input struct {
				WorkoutID string `json:"workout_id" binding:"required"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				badRequest(c)
				return
			}

			session, err := sessionRepo.StartSession(c.Request.Context(), userID(c), input.WorkoutID)
			if err != nil {
				notFoundOr(c, err, "Workout not found", "Failed to start session")
				return
			}
			c.JSON(http.StatusCreated, session)
		})

		authAPI.GET("/sessions/active", func(c *gin.Context) {
			session, err := sessionRepo.GetActiveSessionWithExercises(c.Request.Context(), userID(c))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "No active session"})
				return
			}
			c.JSON(http.StatusOK, session)
		})

		authAPI.PUT("/sessions/:id/end", func(c *gin.Context) {
			session, err := sessionRepo.EndSession(c.Request.Context(), userID(c), c.Param("id"))
			if err != nil {
				notFoundOr(c, err, "Session not found", "Failed to end session")
				return
			}
			c.JSON(http.StatusOK, session)
		})

		// Session exercise routes
		authAPI.POST("/sessions/:id/exercises", func(c *gin.Context) {
			var input struct {
				ExerciseID string `json:"exerciseId" binding:"required"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				badRequest(c)
				return
			}
			sessionExercise, err := sessionRepo.CreateSessionExercise(c.Request.Context(), userID(c), c.Param("id"), input.ExerciseID)
			if err != nil {
				notFoundOr(c, err, "Session or exercise not found", "Failed to add exercise to session")
				return
			}
			c.JSON(http.StatusCreated, sessionExercise)
		})

		// Exercise set routes
		authAPI.POST("/exercise-sets", func(c *gin.Context) {
			var input struct {
				SessionExerciseID string  `json:"sessionExerciseId" binding:"required"`
				Reps              int     `json:"reps" binding:"min=0,max=10000"`
				Weight            float64 `json:"weight" binding:"min=0,max=99999"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				badRequest(c)
				return
			}

			set := &models.ExerciseSet{
				SessionExerciseID: input.SessionExerciseID,
				Reps:              input.Reps,
				Weight:            input.Weight,
			}

			err := sessionRepo.CreateExerciseSet(c.Request.Context(), userID(c), set)
			if err != nil {
				notFoundOr(c, err, "Session exercise not found", "Failed to add set")
				return
			}
			c.JSON(http.StatusCreated, set)
		})

		// Partial update from the set rows: any of reps, weight, completed.
		authAPI.PATCH("/exercise-sets/:id", func(c *gin.Context) {
			var input struct {
				Reps      *int     `json:"reps" binding:"omitempty,min=0,max=10000"`
				Weight    *float64 `json:"weight" binding:"omitempty,min=0,max=99999"`
				Completed *bool    `json:"completed"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				badRequest(c)
				return
			}
			set, err := sessionRepo.PatchExerciseSet(c.Request.Context(), userID(c), c.Param("id"), repository.SetPatch{
				Reps: input.Reps, Weight: input.Weight, Completed: input.Completed,
			})
			if err != nil {
				notFoundOr(c, err, "Set not found", "Failed to update set")
				return
			}
			c.JSON(http.StatusOK, set)
		})

		authAPI.DELETE("/exercise-sets/:id", func(c *gin.Context) {
			if err := sessionRepo.DeleteExerciseSet(c.Request.Context(), userID(c), c.Param("id")); err != nil {
				notFoundOr(c, err, "Set not found", "Failed to delete set")
				return
			}
			c.Status(http.StatusNoContent)
		})

		authAPI.PUT("/exercise-sets/:id/complete", func(c *gin.Context) {
			var input struct {
				SetIndex int `json:"setIndex"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				badRequest(c)
				return
			}
			err := sessionRepo.CompleteExerciseSet(c.Request.Context(), userID(c), c.Param("id"), input.SetIndex)
			if errors.Is(err, repository.ErrInvalidSetIndex) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid set index"})
				return
			}
			if err != nil {
				notFoundOr(c, err, "Set not found", "Failed to complete set")
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Set completed"})
		})

		authAPI.PUT("/exercise-sets/:id", func(c *gin.Context) {
			var input struct {
				Reps   int     `json:"reps" binding:"required,min=1"`
				Weight float64 `json:"weight" binding:"required,min=0.01"`
				Notes  *string `json:"notes"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				badRequest(c)
				return
			}
			set := &models.ExerciseSet{
				ID:        c.Param("id"),
				Reps:      input.Reps,
				Weight:    input.Weight,
				Notes:     input.Notes,
				Completed: true,
			}
			err := sessionRepo.UpdateExerciseSet(c.Request.Context(), userID(c), set)
			if err != nil {
				notFoundOr(c, err, "Set not found", "Failed to update set")
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Set updated"})
		})

		// Workout history routes
		authAPI.GET("/sessions/completed", func(c *gin.Context) {
			sessions, err := sessionRepo.GetCompletedSessions(c.Request.Context(), userID(c))
			if err != nil {
				serverError(c, "Failed to fetch workout history", err)
				return
			}
			c.JSON(http.StatusOK, sessions)
		})

		// Progress routes
		authAPI.GET("/progress", func(c *gin.Context) {
			progress, err := sessionRepo.GetProgressData(c.Request.Context(), userID(c))
			if err != nil {
				serverError(c, "Failed to fetch progress", err)
				return
			}
			c.JSON(http.StatusOK, progress)
		})

		// Dino game routes
		authAPI.POST("/dino-game/score", func(c *gin.Context) {
			var input struct {
				Score int `json:"score" binding:"required"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				badRequest(c)
				return
			}

			score, err := workoutRepo.CreateDinoGameScore(c.Request.Context(), userID(c), input.Score)
			if err != nil {
				serverError(c, "Failed to save score", err)
				return
			}
			c.JSON(http.StatusCreated, score)
		})

		authAPI.GET("/dino-game/high-score", func(c *gin.Context) {
			highScore, err := workoutRepo.GetDinoGameHighScore(c.Request.Context(), userID(c))
			if err != nil {
				serverError(c, "Failed to fetch high score", err)
				return
			}
			c.JSON(http.StatusOK, gin.H{"highScore": highScore})
		})
	}

	// Health check
	// Liveness: the process is up even when the database isn't (see /api/status).
	r.GET("/health", func(c *gin.Context) {
		dbStatus := "ok"
		if !db.Ready() {
			dbStatus = "unavailable"
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "database": dbStatus})
	})

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Printf("API available at http://localhost:%s/api", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
