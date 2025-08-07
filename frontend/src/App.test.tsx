import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from './App'

describe('App Component', () => {
  it('renders the app header correctly', () => {
    render(<App />)
    
    expect(screen.getByText('🏋️ Liftoff')).toBeInTheDocument()
    expect(screen.getByText('Track your workouts and build strength')).toBeInTheDocument()
  })

  it('renders create workout section', () => {
    render(<App />)
    
    expect(screen.getByText('Create New Workout')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Workout name (e.g., Push Day)')).toBeInTheDocument()
    expect(screen.getByText('Create Workout')).toBeInTheDocument()
  })

  it('renders workouts section', () => {
    render(<App />)
    
    expect(screen.getByText('Your Workouts')).toBeInTheDocument()
    expect(screen.getByText('No workouts yet. Create your first workout above!')).toBeInTheDocument()
  })

  it('renders select workout message when no workout is selected', () => {
    render(<App />)
    
    expect(screen.getByText('Select a Workout')).toBeInTheDocument()
    expect(screen.getByText('Choose a workout from the left panel to start tracking your exercises.')).toBeInTheDocument()
  })

  describe('Workout Creation', () => {
    it('creates a new workout when form is submitted', async () => {
      const user = userEvent.setup()
      render(<App />)
      
      const workoutInput = screen.getByPlaceholderText('Workout name (e.g., Push Day)')
      const createButton = screen.getByText('Create Workout')
      
      await user.type(workoutInput, 'Push Day')
      await user.click(createButton)
      
      expect(screen.getByText('Push Day')).toBeInTheDocument()
      expect(screen.getByText('0 exercises')).toBeInTheDocument()
    })

    it('creates workout when Enter is pressed', async () => {
      const user = userEvent.setup()
      render(<App />)
      
      const workoutInput = screen.getByPlaceholderText('Workout name (e.g., Push Day)')
      
      await user.type(workoutInput, 'Pull Day{enter}')
      
      expect(screen.getByText('Pull Day')).toBeInTheDocument()
    })

    it('does not create workout with empty name', async () => {
      const user = userEvent.setup()
      render(<App />)
      
      const createButton = screen.getByText('Create Workout')
      
      await user.click(createButton)
      
      expect(screen.getByText('No workouts yet. Create your first workout above!')).toBeInTheDocument()
    })
  })

  describe('Workout Selection', () => {
    it('allows selecting a workout', async () => {
      const user = userEvent.setup()
      render(<App />)
      
      // Create a workout first
      const workoutInput = screen.getByPlaceholderText('Workout name (e.g., Push Day)')
      const createButton = screen.getByText('Create Workout')
      
      await user.type(workoutInput, 'Push Day')
      await user.click(createButton)
      
      // Select the workout
      const startButton = screen.getByText('Start')
      await user.click(startButton)
      
      expect(screen.getByText('Current Workout: Push Day')).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Add Exercise' })).toBeInTheDocument()
    })

    it('shows continue button for selected workout', async () => {
      const user = userEvent.setup()
      render(<App />)
      
      // Create and select a workout
      const workoutInput = screen.getByPlaceholderText('Workout name (e.g., Push Day)')
      const createButton = screen.getByText('Create Workout')
      
      await user.type(workoutInput, 'Push Day')
      await user.click(createButton)
      
      const startButton = screen.getByText('Start')
      await user.click(startButton)
      
      // Button should now say "Continue"
      expect(screen.getByText('Continue')).toBeInTheDocument()
    })
  })

  describe('Exercise Management', () => {
    beforeEach(async () => {
      const user = userEvent.setup()
      render(<App />)
      
      // Create and select a workout
      const workoutInput = screen.getByPlaceholderText('Workout name (e.g., Push Day)')
      const createButton = screen.getByText('Create Workout')
      
      await user.type(workoutInput, 'Push Day')
      await user.click(createButton)
      
      const startButton = screen.getByText('Start')
      await user.click(startButton)
    })

    it('renders exercise form when workout is selected', () => {
      expect(screen.getByRole('heading', { name: 'Add Exercise' })).toBeInTheDocument()
      expect(screen.getByPlaceholderText('Exercise name')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('Sets')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('Reps')).toBeInTheDocument()
      expect(screen.getByPlaceholderText('Weight (lbs)')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Add Exercise' })).toBeInTheDocument()
    })

    it('adds an exercise to the workout', async () => {
      const user = userEvent.setup()
      
      const exerciseNameInput = screen.getByPlaceholderText('Exercise name')
      const setsInput = screen.getByPlaceholderText('Sets')
      const repsInput = screen.getByPlaceholderText('Reps')
      const weightInput = screen.getByPlaceholderText('Weight (lbs)')
      const addButton = screen.getByRole('button', { name: 'Add Exercise' })
      
      await user.type(exerciseNameInput, 'Bench Press')
      await user.clear(setsInput)
      await user.type(setsInput, '3')
      await user.clear(repsInput)
      await user.type(repsInput, '10')
      await user.clear(weightInput)
      await user.type(weightInput, '135')
      await user.click(addButton)
      
      expect(screen.getByText('Bench Press')).toBeInTheDocument()
      expect(screen.getByText('3 sets × 10 reps')).toBeInTheDocument()
      expect(screen.getByText('135 lbs')).toBeInTheDocument()
    })

    it('does not add exercise with empty name', async () => {
      const user = userEvent.setup()
      
      const addButton = screen.getByRole('button', { name: 'Add Exercise' })
      await user.click(addButton)
      
      expect(screen.getByText('No exercises added yet. Add your first exercise above!')).toBeInTheDocument()
    })

    it('resets exercise form after adding exercise', async () => {
      const user = userEvent.setup()
      
      const exerciseNameInput = screen.getByPlaceholderText('Exercise name')
      const addButton = screen.getByRole('button', { name: 'Add Exercise' })
      
      await user.type(exerciseNameInput, 'Bench Press')
      await user.click(addButton)
      
      // Form should be reset
      expect(screen.getByPlaceholderText('Exercise name')).toHaveValue('')
    })

    it('shows exercise count in workout card', async () => {
      const user = userEvent.setup()
      
      const exerciseNameInput = screen.getByPlaceholderText('Exercise name')
      const addButton = screen.getByRole('button', { name: 'Add Exercise' })
      
      await user.type(exerciseNameInput, 'Bench Press')
      await user.click(addButton)
      
      // Check that the workout card shows 1 exercise
      expect(screen.getByText('1 exercises')).toBeInTheDocument()
    })
  })

  describe('Multiple Workouts', () => {
    it('manages multiple workouts correctly', async () => {
      const user = userEvent.setup()
      render(<App />)
      
      // Create first workout
      const workoutInput = screen.getByPlaceholderText('Workout name (e.g., Push Day)')
      const createButton = screen.getByText('Create Workout')
      
      await user.type(workoutInput, 'Push Day')
      await user.click(createButton)
      
      // Create second workout
      await user.type(workoutInput, 'Pull Day')
      await user.click(createButton)
      
      // Both workouts should be visible
      expect(screen.getByText('Push Day')).toBeInTheDocument()
      expect(screen.getByText('Pull Day')).toBeInTheDocument()
      expect(screen.getAllByText('0 exercises')).toHaveLength(2)
    })

    it('switches between workouts correctly', async () => {
      const user = userEvent.setup()
      render(<App />)
      
      // Create two workouts
      const workoutInput = screen.getByPlaceholderText('Workout name (e.g., Push Day)')
      const createButton = screen.getByText('Create Workout')
      
      await user.type(workoutInput, 'Push Day')
      await user.click(createButton)
      
      await user.type(workoutInput, 'Pull Day')
      await user.click(createButton)
      
      // Select first workout
      const startButtons = screen.getAllByText('Start')
      await user.click(startButtons[0])
      
      expect(screen.getByText(/Current Workout: Push Day/)).toBeInTheDocument()
      
      // Switch to second workout
      const continueButtons = screen.getAllByText('Start')
      await user.click(continueButtons[1])
      
      // Check that Pull Day is now the current workout
      expect(screen.getByText('Pull Day')).toBeInTheDocument()
    })
  })

  describe('Exercise Display', () => {
    beforeEach(async () => {
      const user = userEvent.setup()
      render(<App />)
      
      // Create and select a workout
      const workoutInput = screen.getByPlaceholderText('Workout name (e.g., Push Day)')
      const createButton = screen.getByText('Create Workout')
      
      await user.type(workoutInput, 'Push Day')
      await user.click(createButton)
      
      const startButton = screen.getByText('Start')
      await user.click(startButton)
    })

    it('displays exercises with correct information', async () => {
      const user = userEvent.setup()
      
      const exerciseNameInput = screen.getByPlaceholderText('Exercise name')
      const setsInput = screen.getByPlaceholderText('Sets')
      const repsInput = screen.getByPlaceholderText('Reps')
      const weightInput = screen.getByPlaceholderText('Weight (lbs)')
      const addButton = screen.getByRole('button', { name: 'Add Exercise' })
      
      await user.type(exerciseNameInput, 'Bench Press')
      await user.clear(setsInput)
      await user.type(setsInput, '4')
      await user.clear(repsInput)
      await user.type(repsInput, '8')
      await user.clear(weightInput)
      await user.type(weightInput, '185')
      await user.click(addButton)
      
      expect(screen.getByText('Bench Press')).toBeInTheDocument()
      expect(screen.getByText('4 sets × 8 reps')).toBeInTheDocument()
      expect(screen.getByText('185 lbs')).toBeInTheDocument()
    })

    it('does not show weight when weight is 0', async () => {
      const user = userEvent.setup()
      
      const exerciseNameInput = screen.getByPlaceholderText('Exercise name')
      const addButton = screen.getByRole('button', { name: 'Add Exercise' })
      
      await user.type(exerciseNameInput, 'Push-ups')
      await user.click(addButton)
      
      expect(screen.getByText('Push-ups')).toBeInTheDocument()
      expect(screen.getByText('3 sets × 10 reps')).toBeInTheDocument()
      expect(screen.queryByText('0 lbs')).not.toBeInTheDocument()
    })
  })
})
