# Routines Architecture Plan

## Overview

Extend Liftoff to support both **individual workouts** and **multi-workout routines** (e.g., Push Pull Legs, Upper Lower, Upper Lower 4-Day). A routine is an ordered sequence of workouts that users can follow as a program.

## Data Model

### New Entities

**Routine**
- `id`, `user_id`, `name`, `description` (optional), `created_at`, `updated_at`
- A named program (e.g., "Push Pull Legs")

**RoutineWorkout**
- `id`, `routine_id`, `workout_id`, `slot_order` (1, 2, 3...)
- Links workouts to a routine with ordering
- A workout can be reused across multiple routines

### Relationships

```
User 1──* Routine 1──* RoutineWorkout *──1 Workout
                                    (slot_order)
```

- Workouts remain standalone; routines reference them
- Deleting a routine does not delete its workouts
- Deleting a workout removes it from any routines that reference it (CASCADE or soft-remove from routine_workouts)

## Session Tracking

- **No change** to WorkoutSession. When user starts "Day 2 of PPL", we start a WorkoutSession for the workout in slot 2.
- The routine is a selection/organization layer; sessions are still per-workout.

## Routine Templates

Predefined templates (in code) that users can instantiate:

| Template        | Workouts                    | Description                    |
|----------------|-----------------------------|--------------------------------|
| Push Pull Legs | Push, Pull, Legs            | Classic 3-day split           |
| Upper Lower    | Upper, Lower                | 2-day split                   |
| Upper Lower 4-Day | Upper A, Lower A, Upper B, Lower B | 4-day variation          |
| Full Body      | Full Body                   | Single workout (1-day)        |

When user "creates from template":
1. Create each workout with exercises
2. Create the routine
3. Link workouts to routine via routine_workouts

## API Design

```
GET    /api/routines              List user's routines (with workout summaries)
POST   /api/routines              Create routine (body: name, workout_ids in order)
GET    /api/routines/:id          Get routine with full workouts + exercises
PUT    /api/routines/:id          Update routine (name, reorder workouts)
DELETE /api/routines/:id          Delete routine (workouts remain)
GET    /api/routines/templates    List available templates (metadata only)
POST   /api/routines/from-template/:templateId   Create routine + workouts from template
```

## UI Flow

1. **Routines tab** (new): List routines, create, edit, delete
2. **Create routine**: Manual (pick existing workouts, order) or From template
3. **Start workout**: 
   - From Workouts view: start any standalone workout (existing)
   - From Routines view: pick routine → pick day (1, 2, 3...) → start that workout's session

## Considerations

- **Backward compatibility**: Existing workouts and sessions unchanged
- **Orphaned workouts**: If user deletes a routine, workouts stay; if user deletes a workout, remove from routine_workouts (or CASCADE)
- **Templates are code-defined**: No DB table for templates; easy to add new ones
- **Sample data**: New users can optionally add sample routines from templates on first load (or via explicit "Add sample routines" action)
