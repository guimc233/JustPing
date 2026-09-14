import { useState, useEffect } from 'react'
import { Navbar } from './components/Navbar'
import { SetupWizard } from './components/SetupWizard'
import { PublicDashboard } from './components/PublicDashboard'
import { AdminPanel } from './components/AdminPanel'
import { ShieldAlert, X } from 'lucide-react'

export function App() {
  const [setupStatus, setSetupStatus] = useState<any>(null)
  const [currentUser, setCurrentUser] = useState<any>({ authenticated: false })
  const [activeTab, setActiveTab] = useState<string>('dashboard')
  const [loading, setLoading] = useState(true)
  const [loginError, setLoginError] = useState<string | null>(null)

  // Check URL parameters for OAuth errors
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const error = params.get('error')
    const email = params.get('email')
    if (error === 'email_not_whitelisted') {
      setLoginError(
        `Login Rejected: Your verified GitHub email (${email || 'unknown'}) is not in the whitelist. Please contact the administrator.`
      )
    } else if (error) {
      setLoginError(`Authentication failed: ${error}`)
    }
  }, [])

  const checkStatus = async () => {
    try {
      const [setupRes, authRes] = await Promise.all([
        fetch('/api/setup/status'),
        fetch('/api/auth/me'),
      ])

      const setupData = await setupRes.json()
      const authData = await authRes.json()

      setSetupStatus(setupData)
      setCurrentUser(authData)

      // If already authenticated and URL was /admin, jump to admin tab
      if (authData.authenticated && window.location.pathname.startsWith('/admin')) {
        setActiveTab('admin')
      }
    } catch (err) {
      console.error('Failed to initialize platform status:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    checkStatus()
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

  // 1. Initial Setup Wizard required
  if (setupStatus && !setupStatus.initialized) {
    return (
      <SetupWizard
        appUrl={setupStatus.app_url}
        callbackUrl={setupStatus.oauth_callback_url}
        onComplete={checkStatus}
      />
    )
  }

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col">
      <Navbar
        currentUser={currentUser}
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
        <div className="flex items-center justify-center gap-1">
          <span>Powered by</span>
          <span className="font-semibold text-foreground">JustPing</span>
          <span>• Distributed Latency & Quality Observability</span>
        </div>
      </footer>
    </div>
  )
}

export default App
