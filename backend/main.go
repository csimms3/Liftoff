package main

import (
	"log"
	"net/http"
	"os"

	"liftoff/backend/auth"
	"liftoff/backend/database"
	"liftoff/backend/handlers"
	"liftoff/backend/models"
	"liftoff/backend/repository"

	"github.com/gin-gonic/gin"
)

// Liftoff API Server
// A workout tracking application with Go backend and React frontend
//
// Features:
// - Workout management (create, read, update, delete)
// - Exercise tracking with sets, reps, and weights
// - Workout sessions and progress tracking
// - Exercise templates for quick workout building
// - Support for both PostgreSQL and SQLite databases

func main() {
	// Initialize database connection
	db, err := database.NewDatabase()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize repositories for data access
	workoutRepo := repository.NewWorkoutRepository(db.GetPool(), db.GetSQLite(), db.IsSQLite())
	sessionRepo := repository.NewSessionRepository(db.GetPool(), db.GetSQLite(), db.IsSQLite())
	userRepo := repository.NewUserRepository(db.GetPool(), db.GetSQLite(), db.IsSQLite())
	authHandler := handlers.NewAuthHandler(userRepo)

	// Setup Gin router with default middleware (Logger and Recovery)
	r := gin.Default()

	// Add CORS middleware for frontend integration
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// API routes group - all endpoints under /api
	api := r.Group("/api")
	{
		// Auth routes (no middleware required for login/register)
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/register", authHandler.Register)
		api.POST("/auth/forgot-password", authHandler.ForgotPassword)
		api.POST("/auth/reset-password", authHandler.ResetPassword)
		api.GET("/auth/me", auth.AuthMiddleware(), authHandler.Me)

		// Protected routes - add auth middleware group
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

		// Workout template routes
		api.GET("/workout-templates", func(c *gin.Context) {
			templates, err := workoutRepo.GetWorkoutTemplates(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, templates)
		})

		api.GET("/exercise-templates", func(c *gin.Context) {
			templates, err := workoutRepo.GetExerciseTemplates(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, templates)
		})

		authAPI.POST("/workout-templates/:id/create", func(c *gin.Context) {
			var req struct {
				Name string `json:"name"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			workout, err := workoutRepo.CreateWorkoutFromTemplate(c.Request.Context(), userID(c), c.Param("id"), req.Name)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, exercise)
		})

		authAPI.DELETE("/exercises/:id", func(c *gin.Context) {
			err := workoutRepo.DeleteExercise(c.Request.Context(), userID(c), c.Param("id"))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			session, err := sessionRepo.CreateSessionWithExercises(c.Request.Context(), userID(c), input.WorkoutID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			sessionExercise, err := sessionRepo.CreateSessionExercise(c.Request.Context(), userID(c), c.Param("id"), input.ExerciseID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, sessionExercise)
		})

		// Exercise set routes
		authAPI.POST("/exercise-sets", func(c *gin.Context) {
			var input struct {
				SessionExerciseID string  `json:"sessionExerciseId" binding:"required"`
				Reps              int     `json:"reps"`
				Weight            float64 `json:"weight"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			set := &models.ExerciseSet{
				SessionExerciseID: input.SessionExerciseID,
				Reps:              input.Reps,
				Weight:            input.Weight,
			}

			err := sessionRepo.CreateExerciseSet(c.Request.Context(), userID(c), set)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, set)
		})

		authAPI.PUT("/exercise-sets/:id/complete", func(c *gin.Context) {
			var input struct {
				SetIndex int `json:"setIndex"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			err := sessionRepo.CompleteExerciseSet(c.Request.Context(), userID(c), c.Param("id"), input.SetIndex)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Set updated"})
		})

		// Workout history routes
		authAPI.GET("/sessions/completed", func(c *gin.Context) {
			sessions, err := sessionRepo.GetCompletedSessions(c.Request.Context(), userID(c))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, sessions)
		})

		// Progress routes
		authAPI.GET("/progress", func(c *gin.Context) {
			progress, err := sessionRepo.GetProgressData(c.Request.Context(), userID(c))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			score, err := workoutRepo.CreateDinoGameScore(c.Request.Context(), userID(c), input.Score)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, score)
		})

		authAPI.GET("/dino-game/high-score", func(c *gin.Context) {
			highScore, err := workoutRepo.GetDinoGameHighScore(c.Request.Context(), userID(c))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"highScore": highScore})
		})
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
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
