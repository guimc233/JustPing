import React, { useState } from 'react'
import { Activity, ShieldCheck, LogIn, LogOut, Radio, Settings, ChevronDown, Menu, X } from 'lucide-react'
import { Button } from './ui/button'

interface NavbarProps {
  currentUser: {
    authenticated: boolean
    user?: {
      id: number
      username: string
      email: string
      avatar_url: string
      role: string
    }
  }
  setupStatus: {
    github_configured?: boolean
    google_configured?: boolean
    oidc_configured?: boolean
  } | null
  activeTab: string
  setActiveTab: (tab: string) => void
  onLogout: () => void
}

export const Navbar: React.FC<NavbarProps> = ({
  currentUser,
  setupStatus,
  activeTab,
  setActiveTab,
  onLogout,
}) => {
  const [showLoginMenu, setShowLoginMenu] = useState(false)
  const [showMobileMenu, setShowMobileMenu] = useState(false)

  const providers: { name: string; url: string }[] = []
  if (setupStatus?.github_configured !== false) {
    providers.push({ name: 'GitHub', url: '/api/auth/github/login' })
  }
  if (setupStatus?.google_configured) {
    providers.push({ name: 'Google', url: '/api/auth/google/login' })
  }
  if (setupStatus?.oidc_configured) {
    providers.push({ name: 'OIDC / SSO', url: '/api/auth/oidc/login' })
  }
  if (providers.length === 0) {
    providers.push({ name: 'Login', url: '/api/auth/github/login' })
  }

  return (
    <header className="sticky top-0 z-50 w-full border-b border-border/60 bg-background/95 backdrop-blur-sm">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6">
        {/* Logo & Brand */}
        <div className="flex items-center gap-6">
          <button
            onClick={() => {
              setActiveTab('dashboard')
              setShowMobileMenu(false)
            }}
            className="flex items-center gap-2 font-bold tracking-tight text-foreground transition hover:opacity-80 cursor-pointer"
          >
            <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
              <Activity className="size-4" />
            </div>
            <span className="text-lg">JustPing</span>
            <span className="relative flex size-2">
              <span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-75"></span>
              <span className="relative inline-flex size-2 rounded-full bg-emerald-500"></span>
            </span>
          </button>

          {/* Desktop Navigation Links */}
          <nav className="hidden md:flex items-center gap-1 text-sm">
            <Button
              variant={activeTab === 'dashboard' ? 'secondary' : 'ghost'}
              size="sm"
              onClick={() => setActiveTab('dashboard')}
            >
              <Activity className="size-4 mr-1.5 text-primary" />
              Probes
            </Button>
            <Button
              variant={activeTab === 'matrix' ? 'secondary' : 'ghost'}
              size="sm"
              onClick={() => setActiveTab('matrix')}
            >
              <Radio className="size-4 mr-1.5 text-blue-400" />
              Matrix
            </Button>
            {currentUser.authenticated && (
              <Button
                variant={activeTab === 'admin' ? 'secondary' : 'ghost'}
                size="sm"
                onClick={() => setActiveTab('admin')}
              >
                <ShieldCheck className="size-4 mr-1.5 text-amber-400" />
                Admin Panel
              </Button>
            )}
          </nav>
        </div>

        {/* User / Login status & Mobile Toggle */}
        <div className="flex items-center gap-2">
          {currentUser.authenticated ? (
            <div className="flex items-center gap-2">
              <div className="hidden sm:flex items-center gap-2 text-right">
                {currentUser.user?.avatar_url && (
                  <img
                    src={currentUser.user.avatar_url}
                    alt={currentUser.user.username}
                    className="size-7 rounded-full border border-border"
                  />
                )}
                <div>
                  <div className="text-xs font-semibold leading-tight">{currentUser.user?.username}</div>
                  <div className="text-[10px] text-muted-foreground uppercase">{currentUser.user?.role}</div>
                </div>
              </div>

              <Button
                variant="outline"
                size="sm"
                className="hidden sm:flex"
                onClick={() => setActiveTab('admin')}
              >
                <Settings className="size-3.5 mr-1" />
                Manage
              </Button>

              <Button
                variant="ghost"
                size="sm"
                onClick={onLogout}
                className="text-muted-foreground hover:text-destructive"
              >
                <LogOut className="size-4" />
                <span className="hidden sm:inline ml-1.5">Logout</span>
              </Button>
            </div>
          ) : providers.length === 1 ? (
            <Button
              variant="default"
              size="sm"
              onClick={() => (window.location.href = providers[0].url)}
              className="font-medium"
            >
              <LogIn className="size-4 mr-1.5" />
              {providers[0].name} Login
            </Button>
          ) : (
            <div className="relative">
              <Button
                variant="default"
                size="sm"
                onClick={() => setShowLoginMenu(!showLoginMenu)}
                className="font-medium"
              >
                <LogIn className="size-4 mr-1.5" />
                Sign In
                <ChevronDown className="size-3.5 ml-1" />
              </Button>

              {showLoginMenu && (
                <div className="absolute right-0 mt-2 w-40 rounded-lg border border-border bg-popover p-1 shadow-lg z-50">
                  {providers.map((p) => (
                    <button
                      key={p.name}
                      onClick={() => (window.location.href = p.url)}
                      className="w-full text-left rounded-md px-3 py-1.5 text-xs font-medium hover:bg-accent hover:text-accent-foreground transition cursor-pointer"
                    >
                      {p.name}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Mobile hamburger button */}
          <Button
            variant="ghost"
            size="icon"
            className="md:hidden size-8"
            onClick={() => setShowMobileMenu(!showMobileMenu)}
          >
            {showMobileMenu ? <X className="size-4" /> : <Menu className="size-4" />}
          </Button>
        </div>
      </div>

      {/* Mobile dropdown menu */}
      {showMobileMenu && (
        <div className="md:hidden border-t border-border/60 bg-background/95 px-4 py-3 space-y-1">
          <Button
            variant={activeTab === 'dashboard' ? 'secondary' : 'ghost'}
            size="sm"
            className="w-full justify-start"
            onClick={() => {
              setActiveTab('dashboard')
              setShowMobileMenu(false)
            }}
          >
            <Activity className="size-4 mr-2 text-primary" />
            Probes
          </Button>
          <Button
            variant={activeTab === 'matrix' ? 'secondary' : 'ghost'}
            size="sm"
            className="w-full justify-start"
            onClick={() => {
              setActiveTab('matrix')
              setShowMobileMenu(false)
            }}
          >
            <Radio className="size-4 mr-2 text-blue-400" />
            Matrix
          </Button>
          {currentUser.authenticated && (
            <Button
              variant={activeTab === 'admin' ? 'secondary' : 'ghost'}
              size="sm"
              className="w-full justify-start"
              onClick={() => {
                setActiveTab('admin')
                setShowMobileMenu(false)
              }}
            >
              <ShieldCheck className="size-4 mr-2 text-amber-400" />
              Admin Panel
            </Button>
          )}
        </div>
      )}
    </header>
  )
}
