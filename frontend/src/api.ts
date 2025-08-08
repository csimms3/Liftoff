const API_BASE = 'http://localhost:8080/api'

export interface Workout {
  id: string
  name: string
  exercises: Exercise[]
  createdAt: string
}

export interface Exercise {
  id: string
  name: string
  sets: number
  reps: number
  weight: number
  workoutId: string
}

export interface WorkoutSession {
  id: string
  workoutId: string
  workout: Workout
  startedAt: string
  endedAt?: string
  isActive: boolean
  exercises: SessionExercise[]
}

export interface SessionExercise {
  id: string
  exerciseId: string
  exercise: Exercise
  sets: ExerciseSet[]
}

export interface ExerciseSet {
  id: string
  reps: number
  weight: number
  completed: boolean
  notes?: string
}

class ApiService {
  private async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
    try {
      const response = await fetch(`${API_BASE}${endpoint}`, {
        headers: {
          'Content-Type': 'application/json',
          ...options?.headers,
        },
        ...options,
      })

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }

      return response.json()
    } catch (error) {
      throw error
    }
  }

  // Workout endpoints
  async getWorkouts(): Promise<Workout[]> {
    return this.request<Workout[]>('/workouts')
  }

  async createWorkout(name: string): Promise<Workout> {
    return this.request<Workout>('/workouts', {
      method: 'POST',
      body: JSON.stringify({ name }),
    })
  }

  async getWorkout(id: string): Promise<Workout> {
    return this.request<Workout>(`/workouts/${id}`)
  }

  // Exercise endpoints
  async createExercise(exercise: Omit<Exercise, 'id'>): Promise<Exercise> {
    return this.request<Exercise>('/exercises', {
      method: 'POST',
      body: JSON.stringify(exercise),
    })
  }

  async getExercisesByWorkout(workoutId: string): Promise<Exercise[]> {
    return this.request<Exercise[]>(`/workouts/${workoutId}/exercises`)
  }

  // Session endpoints
  async createSession(workoutId: string): Promise<WorkoutSession> {
    return this.request<WorkoutSession>('/sessions', {
      method: 'POST',
      body: JSON.stringify({ workoutId }),
    })
  }

  async getActiveSession(): Promise<WorkoutSession | null> {
    try {
      return await this.request<WorkoutSession>('/sessions/active')
    } catch (error) {
      return null
    }
  }

  async endSession(id: string): Promise<WorkoutSession> {
    return this.request<WorkoutSession>(`/sessions/${id}/end`, {
      method: 'PUT',
    })
  }

  async completeSet(sessionExerciseId: string, setIndex: number): Promise<void> {
    return this.request<void>(`/exercise-sets/${sessionExerciseId}/complete`, {
      method: 'PUT',
      body: JSON.stringify({ setIndex }),
    })
  }

  async getProgressData(): Promise<ProgressData[]> {
    return this.request<ProgressData[]>('/progress')
  }

  async deleteWorkout(id: string): Promise<void> {
    return this.request<void>(`/workouts/${id}`, {
      method: 'DELETE',
    })
  }

  async deleteExercise(id: string): Promise<void> {
    return this.request<void>(`/exercises/${id}`, {
      method: 'DELETE',
    })
  }
}

export interface ProgressData {
  exerciseName: string
  date: string
  maxWeight: number
  totalVolume: number
}

export const apiService = new ApiService()
