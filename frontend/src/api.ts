const AUTH_KEY = 'liftoff-auth'

function getAuthToken(): string | null {
  try {
    const stored = localStorage.getItem(AUTH_KEY)
    if (!stored) return null
    const data = JSON.parse(stored)
    return data?.token ?? null
  } catch {
    return null
  }
}

/** Dispatch when API gets 401 - AuthContext listens and logs out */
export function dispatchUnauthorized(): void {
  window.dispatchEvent(new CustomEvent('liftoff:unauthorized'))
}

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
	movement_id?: string;
	created_at: string;
	updated_at: string;
}

export interface WorkoutSession {
	id: string;
	/** Empty once the workout has been deleted; workout_name is kept. */
	workout_id: string;
	workout_name: string;
	workout: Workout;
	started_at: string;
	ended_at?: string;
	is_active: boolean;
	exercises: SessionExercise[];
}

export interface SessionExercise {
	id: string;
	/** Empty once the exercise has been removed from its workout; name is kept. */
	exercise_id: string;
	movement_id: string;
	name: string;
	exercise: Exercise;
	sets: ExerciseSet[];
	/** Completed sets from the last session that logged this exercise, in order. */
	previous?: ExerciseSet[] | null;
}

export interface ExerciseSet {
	id: string;
	reps: number;
	weight: number;
	completed: boolean;
	notes?: string;
}

export interface WorkoutChanges {
	added: string[];
	removed: string[];
	set_counts: { name: string; from: number; to: number }[];
	reordered: boolean;
	has_changes: boolean;
}

export interface SessionSummary {
	session_id: string;
	workout_id: string;
	workout_name: string;
	duration_seconds: number;
	sets_done: number;
	sets_total: number;
	/** weight x reps of completed sets, in lbs */
	volume: number;
	changes: WorkoutChanges;
	can_update_workout: boolean;
}

export interface MovementSummary {
	id: string;
	name: string;
	category: string;
	last_used: string | null;
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
		this.baseUrl = '/api';
	}

  private async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const token = getAuthToken()
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options?.headers as Record<string, string>),
    }
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      ...options,
      headers,
    })

    if (!response.ok) {
      if (response.status === 401) {
        localStorage.removeItem(AUTH_KEY)
        dispatchUnauthorized()
      }
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    if (response.status === 204) {
      return undefined as T
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
      body: JSON.stringify({ workout_id: workoutId }),
    })
  }

  async getActiveSession(): Promise<WorkoutSession | null> {
    try {
      return await this.request<WorkoutSession>('/sessions/active')
    } catch {
      return null
    }
  }

  async getSessionSummary(id: string): Promise<SessionSummary> {
    return this.request<SessionSummary>(`/sessions/${id}/summary`)
  }

  /** Ends the session; with updateWorkout, also saves its structure to the workout. */
  async finishSession(id: string, updateWorkout: boolean): Promise<WorkoutSession> {
    return this.request<WorkoutSession>(`/sessions/${id}/finish`, {
      method: 'POST',
      body: JSON.stringify({ update_workout: updateWorkout }),
    })
  }

  /** Deletes a session with no logged sets. */
  async discardSession(id: string): Promise<void> {
    return this.request<void>(`/sessions/${id}`, { method: 'DELETE' })
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

  async updateSet(setId: string, reps: number, weight: number, notes?: string): Promise<void> {
    return this.request<void>(`/exercise-sets/${setId}`, {
      method: 'PUT',
      body: JSON.stringify({ reps, weight, notes }),
    })
  }

  async addExerciseToSession(sessionId: string, exerciseId: string): Promise<SessionExercise> {
    return this.request<SessionExercise>(`/sessions/${sessionId}/exercises`, {
      method: 'POST',
      body: JSON.stringify({ exerciseId }),
    })
  }

  async createSet(sessionExerciseId: string, reps: number, weight: number): Promise<ExerciseSet> {
    return this.request<ExerciseSet>('/exercise-sets', {
      method: 'POST',
      body: JSON.stringify({ sessionExerciseId, reps, weight }),
    })
  }

  /** Updates any of reps, weight or completed on a set; returns the saved set. */
  async patchSet(setId: string, patch: { reps?: number; weight?: number; completed?: boolean }): Promise<ExerciseSet> {
    return this.request<ExerciseSet>(`/exercise-sets/${setId}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    })
  }

  async deleteSet(setId: string): Promise<void> {
    return this.request<void>(`/exercise-sets/${setId}`, { method: 'DELETE' })
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

	// Workout template methods (no auth required)
	async getWorkoutTemplates(): Promise<WorkoutTemplate[]> {
		const response = await fetch(`${this.baseUrl}/workout-templates`);
		if (!response.ok) {
			throw new Error(`Failed to fetch workout templates: ${response.statusText}`);
		}
		return response.json();
	}

	async createWorkoutFromTemplate(templateId: string, name: string): Promise<Workout> {
		const token = getAuthToken()
		const headers: Record<string, string> = { 'Content-Type': 'application/json' }
		if (token) headers['Authorization'] = `Bearer ${token}`
		const response = await fetch(`${this.baseUrl}/workout-templates/${templateId}/create`, {
			method: 'POST',
			headers,
			body: JSON.stringify({ name }),
		});
		if (!response.ok) {
			throw new Error(`Failed to create workout from template: ${response.statusText}`);
		}
		return response.json();
	}

	async getMovements(): Promise<MovementSummary[]> {
    return this.request<MovementSummary[]>('/movements')
  }

  /** Adds an exercise to the active session by movement id or name (found/created). */
  async addSessionExercise(sessionId: string, target: { movement_id?: string; name?: string }, position?: number): Promise<SessionExercise> {
    return this.request<SessionExercise>(`/sessions/${sessionId}/movements`, {
      method: 'POST',
      body: JSON.stringify({ ...target, ...(position !== undefined && { position }) }),
    })
  }

  async removeSessionExercise(id: string): Promise<void> {
    return this.request<void>(`/sessions/exercises/${id}`, { method: 'DELETE' })
  }

  async moveSessionExercise(id: string, position: number): Promise<void> {
    return this.request<void>(`/sessions/exercises/${id}`, { method: 'PATCH', body: JSON.stringify({ position }) })
  }

  async replaceSessionExercise(id: string, target: { movement_id?: string; name?: string }): Promise<SessionExercise> {
    return this.request<SessionExercise>(`/sessions/exercises/${id}/replace`, { method: 'POST', body: JSON.stringify(target) })
  }

	async getExerciseTemplates(): Promise<ExerciseTemplate[]> {
		return this.request<ExerciseTemplate[]>('/exercise-templates')
	}

	async saveDinoGameScore(score: number): Promise<void> {
		const token = getAuthToken()
		const headers: Record<string, string> = { 'Content-Type': 'application/json' }
		if (token) headers['Authorization'] = `Bearer ${token}`
		const response = await fetch(`${this.baseUrl}/dino-game/score`, {
			method: 'POST',
			headers,
			body: JSON.stringify({ score })
		});
		if (!response.ok) {
			throw new Error('Failed to save dino game score');
		}
	}

	async getDinoGameHighScore(): Promise<number> {
		const token = getAuthToken()
		const headers: Record<string, string> = {}
		if (token) headers['Authorization'] = `Bearer ${token}`
		const response = await fetch(`${this.baseUrl}/dino-game/high-score`, { headers });
		if (!response.ok) {
			throw new Error('Failed to fetch high score');
		}
		const data = await response.json();
		return data.highScore || 0;
	}

	// Routine endpoints
	async getRoutines(): Promise<Routine[]> {
		return this.request<Routine[]>('/routines')
	}

	async getRoutine(id: string): Promise<Routine> {
		return this.request<Routine>(`/routines/${id}`)
	}

	async createRoutine(name: string, description?: string, workoutIds?: string[]): Promise<Routine> {
		return this.request<Routine>('/routines', {
			method: 'POST',
			body: JSON.stringify({ name, description: description || '', workout_ids: workoutIds || [] }),
		})
	}

	async deleteRoutine(id: string): Promise<void> {
		return this.request<void>(`/routines/${id}`, { method: 'DELETE' })
	}

	async getRoutineTemplates(): Promise<RoutineTemplate[]> {
		return this.request<RoutineTemplate[]>('/routine-templates')
	}

	async createRoutineFromTemplate(templateId: string, name?: string): Promise<Routine> {
		return this.request<Routine>(`/routine-templates/${templateId}/create`, {
			method: 'POST',
			body: JSON.stringify({ name: name || '' }),
		})
	}

	// Admin endpoints
	async getAdminUsers(): Promise<AdminUser[]> {
		const data = await this.request<{ users: AdminUser[] }>('/admin/users')
		return data.users
	}

	async getAdminStats(): Promise<AdminStats> {
		return this.request<AdminStats>('/admin/stats')
	}
}

export interface Routine {
	id: string
	name: string
	description: string
	created_at: string
	updated_at: string
	workouts: RoutineWorkout[]
}

export interface RoutineWorkout {
	id: string
	routine_id: string
	workout_id: string
	slot_order: number
	workout?: Workout
}

export interface RoutineTemplate {
	id: string
	name: string
	description: string
	workout_count: number
}

export interface AdminUser {
	id: string
	email: string
	created_at: string
}

export interface AdminStats {
	total_users: number
	total_workouts: number
	total_sessions: number
	new_users_7d: number
}

export interface ProgressData {
  exerciseName: string
  date: string
  maxWeight: number
  totalVolume: number
}

export const apiService = new ApiService()
