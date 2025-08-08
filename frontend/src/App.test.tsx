import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import App from './App'

describe('App', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  test('renders app title', () => {
    render(<App />)
    expect(screen.getByText('🏋️ Liftoff')).toBeInTheDocument()
  })

  test('shows create workout form', async () => {
    render(<App />)
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Workout name...')).toBeInTheDocument()
    })
    expect(screen.getByText('Create')).toBeInTheDocument()
  })

  test('shows loading state initially', async () => {
    render(<App />)
    expect(screen.getByText('Loading workouts...')).toBeInTheDocument()
  })
})
