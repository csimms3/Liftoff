import { render, screen, waitFor } from '@testing-library/react'
import { expect, test } from 'vitest'
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
    await expect(screen.getByPlaceholderText('Workout name...')).toBeInTheDocument()
    
    // Wait for loading to finish and then check for Create button
    await waitFor(() => {
      expect(screen.getByText('Create')).toBeInTheDocument()
    })
  })

  test('shows loading state initially', async () => {
    render(<App />)
    expect(screen.getByText('Loading workouts...')).toBeInTheDocument()
  })
})
