import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { AuthProvider } from './context/AuthContext'
import { AuthGate } from './components/AuthGate'

// Suppress React DevTools comment
if (typeof window !== 'undefined') {
  (window as Window & { __REACT_DEVTOOLS_GLOBAL_HOOK__: { isDisabled: boolean } }).__REACT_DEVTOOLS_GLOBAL_HOOK__ = { isDisabled: true }
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AuthProvider>
      <AuthGate />
    </AuthProvider>
  </StrictMode>,
)
