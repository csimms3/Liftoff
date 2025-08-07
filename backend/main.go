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

func main() {
	// Initialize database
	db, err := database.NewDatabase()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize repositories
	workoutRepo := repository.NewWorkoutRepository(db.GetPool())
	sessionRepo := repository.NewSessionRepository(db.GetPool())

	// Setup Gin router
	r := gin.Default()

	// Add CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// API routes
	api := r.Group("/api")
	{
		// Workout routes
		api.GET("/workouts", func(c *gin.Context) {
			workouts, err := workoutRepo.GetWorkouts(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, workouts)
		})

		api.POST("/workouts", func(c *gin.Context) {
			var input struct {
				Name string `json:"name" binding:"required"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			workout, err := workoutRepo.CreateWorkout(c.Request.Context(), input.Name)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
