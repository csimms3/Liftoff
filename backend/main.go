package main

import (
	"log"
	"net/http"
	"os"

	"liftoff/backend/database"
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
		// Workout management endpoints
		api.GET("/workouts", func(c *gin.Context) {
			workouts, err := workoutRepo.GetWorkouts(c.Request.Context())
			if err != nil {
				log.Printf("Error fetching workouts: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch workouts"})
				return
			}
			c.JSON(http.StatusOK, workouts)
		})

		api.POST("/workouts", func(c *gin.Context) {
			var input struct {
				Name string `json:"name" binding:"required"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Workout name is required"})
				return
			}

			workout, err := workoutRepo.CreateWorkout(c.Request.Context(), input.Name)
			if err != nil {
				log.Printf("Error creating workout: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workout"})
				return
			}
			c.JSON(http.StatusCreated, workout)
		})

		api.GET("/workouts/:id", func(c *gin.Context) {
			id := c.Param("id")
			workout, err := workoutRepo.GetWorkout(c.Request.Context(), id)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Workout not found"})
				return
			}
			c.JSON(http.StatusOK, workout)
		})

		api.DELETE("/workouts/:id", func(c *gin.Context) {
			id := c.Param("id")
			err := workoutRepo.DeleteWorkout(c.Request.Context(), id)
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

		api.POST("/workout-templates/:id/create", func(c *gin.Context) {
			templateID := c.Param("id")
			var req struct {
				Name string `json:"name"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			workout, err := workoutRepo.CreateWorkoutFromTemplate(c.Request.Context(), templateID, req.Name)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, workout)
		})

		// Exercise routes
		api.POST("/exercises", func(c *gin.Context) {
			var input struct {
				Name      string  `json:"name" binding:"required"`
				Sets      int     `json:"sets" binding:"required"`
				Reps      int     `json:"reps" binding:"required"`
				Weight    float64 `json:"weight"`
				WorkoutID string  `json:"workoutId" binding:"required"`
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

			err := workoutRepo.CreateExercise(c.Request.Context(), exercise)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, exercise)
		})

		api.DELETE("/exercises/:id", func(c *gin.Context) {
			id := c.Param("id")
			err := workoutRepo.DeleteExercise(c.Request.Context(), id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Exercise deleted"})
		})

		api.GET("/workouts/:id/exercises", func(c *gin.Context) {
			id := c.Param("id")
			exercises, err := workoutRepo.GetExercisesByWorkout(c.Request.Context(), id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, exercises)
		})

		// Session routes
		api.POST("/sessions", func(c *gin.Context) {
			var input struct {
				WorkoutID string `json:"workoutId" binding:"required"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			session, err := sessionRepo.CreateSession(c.Request.Context(), input.WorkoutID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusCreated, session)
		})

		api.GET("/sessions/active", func(c *gin.Context) {
			session, err := sessionRepo.GetActiveSession(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "No active session"})
				return
			}
			c.JSON(http.StatusOK, session)
		})

		api.PUT("/sessions/:id/end", func(c *gin.Context) {
			id := c.Param("id")
			session, err := sessionRepo.EndSession(c.Request.Context(), id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, session)
		})

		// Exercise set routes
		api.PUT("/exercise-sets/:id/complete", func(c *gin.Context) {
			id := c.Param("id")
			var input struct {
				SetIndex int `json:"setIndex"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			err := sessionRepo.CompleteExerciseSet(c.Request.Context(), id, input.SetIndex)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Set completed"})
		})

		// Progress routes
		api.GET("/progress", func(c *gin.Context) {
			progress, err := sessionRepo.GetProgressData(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, progress)
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
