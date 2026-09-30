# Routines Architecture Plan

## Overview

Extend Liftoff to support both **individual workouts** and **multi-workout routines** (e.g., Push Pull Legs, Upper Lower, Upper Lower 4-Day). A routine is an ordered sequence of workouts that users can follow as a program.

## Data Model

### New Entities

**Routine**
- `id`, `user_id`, `name`, `description` (optional), `created_at`, `updated_at`
- A named program (e.g., "Push Pull Legs")

**Workout ownership** (migration 010; replaces the old `routine_workouts` link table)
- `workouts.routine_id` (NOT NULL, FK, ON DELETE CASCADE) and `workouts.position` (1, 2, 3...)
- Every workout belongs to exactly one routine; the API still reports each routine's workouts as
  `RoutineWorkout` entries (`id` = the workout id, `slot_order` = position)
- `users.current_routine_id` (FK, ON DELETE SET NULL) is the routine the main page shows

### Relationships

```
User 1──* Routine 1──* Workout
                    (position)
```

- A workout is created in a routine (POST /api/workouts takes an optional `routine_id`, defaulting to the current routine)
- Moving a workout into another routine (routine create/update `workout_ids`) moves it out of its old one
- Deleting a routine deletes its workouts; logged sessions keep their name snapshots
- Deleting the current routine makes another of the user's routines current, or none

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
1. Create the routine
2. Create each workout in it, with exercises

## API Design

```
GET    /api/routines              List user's routines (with workout summaries)
POST   /api/routines              Create routine (body: name, workout_ids in order)
GET    /api/routines/:id          Get routine with full workouts + exercises
PUT    /api/routines/:id          Update routine (name, reorder workouts)
DELETE /api/routines/:id          Delete routine and its workouts
GET    /api/routines/current      Current routine id ({"routine_id": id|null})
PUT    /api/routines/current      Set current routine (body: routine_id)
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
- **Orphaned workouts**: none; workouts without a routine were put in a "My Workouts" routine by migration 010
- **Templates are code-defined**: No DB table for templates; easy to add new ones
- **Sample data**: New users can optionally add sample routines from templates on first load (or via explicit "Add sample routines" action)
