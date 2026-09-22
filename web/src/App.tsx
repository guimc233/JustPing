import { useState, useEffect } from 'react'
import { Navbar } from './components/Navbar'
import { SetupWizard } from './components/SetupWizard'
import { PublicDashboard } from './components/PublicDashboard'
import { AdminPanel } from './components/AdminPanel'
import { formatVersion } from './lib/utils'
import { ShieldAlert, X } from 'lucide-react'

export function App() {
  const [setupStatus, setSetupStatus] = useState<any>(null)
  const [currentUser, setCurrentUser] = useState<any>({ authenticated: false })
  const [activeTab, setActiveTab] = useState<string>('dashboard')
  const [loading, setLoading] = useState(true)
  const [loginError, setLoginError] = useState<string | null>(null)
  const [hostVersion, setHostVersion] = useState<string>('')

  // Check URL parameters for OAuth errors
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const error = params.get('error')
    const email = params.get('email')
    if (error === 'email_not_whitelisted') {
      setLoginError(
        `Login Rejected: Your verified email (${email || 'account'}) is not in the whitelist. Please contact the administrator.`
      )
    } else if (error) {
      setLoginError(`Authentication failed: ${error}`)
    }
    if (error || email) {
      window.history.replaceState({}, document.title, window.location.pathname)
    }
  }, [])

  const [statusError, setStatusError] = useState<string | null>(null)

  const checkStatus = async () => {
    try {
      setStatusError(null)
      const [setupRes, authRes] = await Promise.all([
        fetch('/api/setup/status'),
        fetch('/api/auth/me'),
      ])

      if (!setupRes.ok) {
        throw new Error(`Failed to query setup status (HTTP ${setupRes.status})`)
      }

      const setupData = await setupRes.json()
      const authData = await authRes.json().catch(() => ({ authenticated: false }))

      setSetupStatus(setupData)
      setCurrentUser(authData)

      if (authData.authenticated && window.location.pathname.startsWith('/admin')) {
        setActiveTab('admin')
      }
    } catch (err: any) {
      setStatusError(err.message || 'Failed to connect to JustPing host server')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    checkStatus()
    fetch('/api/version')
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data && typeof data.version === 'string') setHostVersion(data.version)
      })
      .catch(() => {
        // Footer version is informational; ignore failures.
      })
  }, [])

  const handleLogout = async () => {
    await fetch('/api/auth/logout', { method: 'POST' })
    setCurrentUser({ authenticated: false })
    setActiveTab('dashboard')
  }

  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background text-sm text-muted-foreground">
        Loading JustPing Platform...
      </div>
    )
  }

  if (statusError) {
    return (
      <div className="flex h-screen flex-col items-center justify-center gap-4 bg-background p-4 text-center">
        <div className="text-destructive font-semibold">Service Unavailable</div>
        <div className="text-xs text-muted-foreground max-w-sm">{statusError}</div>
        <button
          onClick={() => {
            setLoading(true)
            checkStatus()
          }}
          className="rounded-md bg-primary px-4 py-2 text-xs font-medium text-primary-foreground hover:bg-primary/90"
        >
          Retry Connection
        </button>
      </div>
    )
  }

  // 1. Initial Setup Wizard required
  if (setupStatus && !setupStatus.initialized) {
    return (
      <SetupWizard
        appUrl={setupStatus.app_url}
        onComplete={checkStatus}
      />
    )
  }

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col">
      <Navbar
        currentUser={currentUser}
        setupStatus={setupStatus}
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        onLogout={handleLogout}
      />

      <main className="mx-auto max-w-7xl flex-1 px-4 py-6 sm:px-6 w-full">
        {/* Login Error Notification */}
        {loginError && (
          <div className="mb-6 flex items-center justify-between rounded-lg border border-destructive/30 bg-destructive/15 p-4 text-xs text-destructive">
            <div className="flex items-center gap-2 font-medium">
              <ShieldAlert className="size-4 shrink-0" />
              <span>{loginError}</span>
            </div>
            <button
              onClick={() => setLoginError(null)}
              className="rounded p-1 hover:bg-destructive/20 cursor-pointer"
            >
              <X className="size-3.5" />
            </button>
          </div>
        )}

        {/* Tab Views */}
        {(activeTab === 'dashboard' || activeTab === 'matrix' || activeTab === 'agents') && (
          <PublicDashboard isAdmin={currentUser.authenticated} />
        )}

        {activeTab === 'admin' && currentUser.authenticated && (
          <AdminPanel currentUser={currentUser} />
        )}
      </main>

      <footer className="border-t border-border/40 py-6 text-center text-xs text-muted-foreground">
        <div className="flex flex-wrap items-center justify-center gap-1">
          <span>Powered by</span>
          <span className="font-semibold text-foreground">JustPing</span>
          <span>• Distributed Latency &amp; Quality Observability</span>
          {hostVersion && (
            <>
              <span>•</span>
              <span className="font-mono" title="Host build version">
                host {formatVersion(hostVersion)}
              </span>
            </>
          )}
        </div>
      </footer>
    </div>
  )
}

export default App
