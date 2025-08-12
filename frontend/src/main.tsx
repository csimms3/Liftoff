import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'

// Suppress React DevTools comment
if (typeof window !== 'undefined') {
  (window as Window & { __REACT_DEVTOOLS_GLOBAL_HOOK__: { isDisabled: boolean } }).__REACT_DEVTOOLS_GLOBAL_HOOK__ = { isDisabled: true }
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
