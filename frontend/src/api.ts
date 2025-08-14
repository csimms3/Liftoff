const API_BASE = 'http://localhost:8080/api'

// Data model interfaces
export interface Workout {
	id: string;
	name: string;
	type?: string;
	exercises: Exercise[];
	created_at: string;
	updated_at: string;
}

export interface WorkoutTemplate {
	id: string;
	name: string;
	type: string;
	description: string;
	difficulty: string;
	duration: number;
	exercises: Exercise[];
	created_at: string;
}

export interface Exercise {
	id: string;
	name: string;
	sets: number;
	reps: number;
	weight: number;
	workout_id: string;
	created_at: string;
	updated_at: string;
}

export interface WorkoutSession {
	id: string;
	workout_id: string;
	workout: Workout;
	started_at: string;
	ended_at?: string;
	is_active: boolean;
	exercises: SessionExercise[];
}

export interface SessionExercise {
	id: string;
	exercise_id: string;
	exercise: Exercise;
	sets: ExerciseSet[];
}

export interface ExerciseSet {
	id: string;
	reps: number;
	weight: number;
	completed: boolean;
	notes?: string;
}

export interface ExerciseTemplate {
	name: string;
	category: string;
	default_sets: number;
	default_reps: number;
	default_weight: number;
}

export class ApiService {
	private baseUrl: string;

	constructor() {
		this.baseUrl = 'http://localhost:8080/api';
	}

  private async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
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
  	async createExercise(exercise: Omit<Exercise, 'id' | 'created_at' | 'updated_at'>): Promise<Exercise> {
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
    } catch {
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

  // Workout history endpoints
  async getCompletedSessions(): Promise<WorkoutSession[]> {
    return this.request<WorkoutSession[]>('/sessions/completed')
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

	// Workout template methods
	async getWorkoutTemplates(): Promise<WorkoutTemplate[]> {
		const response = await fetch(`${this.baseUrl}/workout-templates`);
		if (!response.ok) {
			throw new Error(`Failed to fetch workout templates: ${response.statusText}`);
		}
		return response.json();
	}

	async createWorkoutFromTemplate(templateId: string, name: string): Promise<Workout> {
		const response = await fetch(`${this.baseUrl}/workout-templates/${templateId}/create`, {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
			},
			body: JSON.stringify({ name }),
		});
		if (!response.ok) {
			throw new Error(`Failed to create workout from template: ${response.statusText}`);
		}
		return response.json();
	}

	async getExerciseTemplates(): Promise<ExerciseTemplate[]> {
		return this.request<ExerciseTemplate[]>('/exercise-templates')
	}
}

export interface ProgressData {
  exerciseName: string
  date: string
  maxWeight: number
  totalVolume: number
}

export const apiService = new ApiService()
