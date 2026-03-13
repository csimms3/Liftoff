import { useState, useEffect } from 'react'

type MenuStyle = 'minimal' | 'bold' | 'stacked'

function getStoredMenuStyle(): MenuStyle {
  const saved = localStorage.getItem('liftoff-menu-style')
  return (saved === 'bold' || saved === 'stacked' ? saved : 'minimal') as MenuStyle
}

interface AuthLayoutProps {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const [menuStyle, setMenuStyle] = useState<MenuStyle>(getStoredMenuStyle)

  useEffect(() => {
    localStorage.setItem('liftoff-menu-style', menuStyle)
  }, [menuStyle])

  return (
    <div className="auth-layout" data-menu-style={menuStyle}>
      <div className="auth-menu-style-swap" role="group" aria-label="Menu style">
        <button
          type="button"
          className={`auth-menu-style-btn ${menuStyle === 'minimal' ? 'active' : ''}`}
          onClick={() => setMenuStyle('minimal')}
          title="Minimal pill"
          aria-pressed={menuStyle === 'minimal'}
        >
          ○
        </button>
        <button
          type="button"
          className={`auth-menu-style-btn ${menuStyle === 'bold' ? 'active' : ''}`}
          onClick={() => setMenuStyle('bold')}
          title="Bold tabs"
          aria-pressed={menuStyle === 'bold'}
        >
          ▬
        </button>
        <button
          type="button"
          className={`auth-menu-style-btn ${menuStyle === 'stacked' ? 'active' : ''}`}
          onClick={() => setMenuStyle('stacked')}
          title="Stacked cards"
          aria-pressed={menuStyle === 'stacked'}
        >
          ▢
        </button>
      </div>
      {children}
    </div>
  )
}
