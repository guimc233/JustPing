import React, { useState } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from './ui/card'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Badge } from './ui/badge'
import { KeyRound, ShieldAlert, ArrowRight, ExternalLink, Copy, Check } from 'lucide-react'

interface SetupWizardProps {
  appUrl: string
  onComplete: () => void
}

export const SetupWizard: React.FC<SetupWizardProps> = ({ appUrl }) => {
  const [provider, setProvider] = useState<'github' | 'google' | 'oidc'>('github')
  const [customAppUrl, setCustomAppUrl] = useState(appUrl || window.location.origin)

  // GitHub
  const [ghClientId, setGhClientId] = useState('')
  const [ghClientSecret, setGhClientSecret] = useState('')

  // Google
  const [googleClientId, setGoogleClientId] = useState('')
  const [googleClientSecret, setGoogleClientSecret] = useState('')

  // OIDC
  const [oidcClientId, setOidcClientId] = useState('')
  const [oidcClientSecret, setOidcClientSecret] = useState('')
  const [oidcAuthUrl, setOidcAuthUrl] = useState('')
  const [oidcTokenUrl, setOidcTokenUrl] = useState('')
  const [oidcUserinfoUrl, setOidcUserinfoUrl] = useState('')
  const [oidcName, setOidcName] = useState('Custom OIDC')

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const normalizedAppUrl = customAppUrl.replace(/\/+$/, '')
  const currentCallbackUrl = `${normalizedAppUrl}/api/auth/${provider}/callback`

  const copyCallback = () => {
    navigator.clipboard.writeText(currentCallbackUrl)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)

    if (provider === 'github' && (!ghClientId.trim() || !ghClientSecret.trim())) {
      setError('GitHub Client ID and Client Secret are required.')
      setLoading(false)
      return
    }
    if (provider === 'google' && (!googleClientId.trim() || !googleClientSecret.trim())) {
      setError('Google Client ID and Client Secret are required.')
      setLoading(false)
      return
    }
    if (
      provider === 'oidc' &&
      (!oidcClientId.trim() || !oidcClientSecret.trim() || !oidcAuthUrl.trim() || !oidcTokenUrl.trim() || !oidcUserinfoUrl.trim())
    ) {
      setError('All OIDC endpoints and credentials are required.')
      setLoading(false)
      return
    }

    try {
      const res = await fetch('/api/setup/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          primary_provider: provider,
          app_url: normalizedAppUrl,
          github_client_id: ghClientId.trim(),
          github_client_secret: ghClientSecret.trim(),
          google_client_id: googleClientId.trim(),
          google_client_secret: googleClientSecret.trim(),
          oidc_client_id: oidcClientId.trim(),
          oidc_client_secret: oidcClientSecret.trim(),
          oidc_auth_url: oidcAuthUrl.trim(),
          oidc_token_url: oidcTokenUrl.trim(),
          oidc_userinfo_url: oidcUserinfoUrl.trim(),
          oidc_name: oidcName.trim(),
        }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Failed to save configuration')
      }

      window.location.href = data.redirect_url || `/api/auth/${provider}/login`
    } catch (err: any) {
      setError(err.message || 'An error occurred during setup')
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-[85vh] items-center justify-center p-4">
      <Card className="w-full max-w-xl border-primary/30 shadow-2xl">
        <CardHeader className="text-center pb-2">
          <div className="mx-auto flex size-12 items-center justify-center rounded-xl bg-primary/10 text-primary mb-2">
            <KeyRound className="size-6" />
          </div>
          <Badge variant="warning" className="mx-auto mb-2">
            Initial Platform Setup
          </Badge>
          <CardTitle className="text-2xl">Welcome to JustPing</CardTitle>
          <CardDescription>
            Configure an OAuth provider. The first user to complete authentication becomes the <strong>Superadmin</strong>.
          </CardDescription>
        </CardHeader>

        <form onSubmit={handleSubmit}>
          <CardContent className="flex flex-col gap-4">
            {error && (
              <div className="flex items-center gap-2 rounded-lg bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20">
                <ShieldAlert className="size-4 shrink-0" />
                <span>{error}</span>
              </div>
            )}

            {/* Provider Switcher */}
            <div className="space-y-1">
              <label className="text-xs font-semibold text-foreground">Select Primary OAuth Provider</label>
              <div className="grid grid-cols-3 gap-2 pt-1">
                {(['github', 'google', 'oidc'] as const).map((p) => (
                  <button
                    key={p}
                    type="button"
                    onClick={() => setProvider(p)}
                    className={`rounded-lg border px-3 py-2 text-xs font-semibold capitalize transition cursor-pointer ${
                      provider === p
                        ? 'border-primary bg-primary text-primary-foreground shadow-sm'
                        : 'border-border bg-card text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    {p === 'oidc' ? 'Generic OIDC' : p}
                  </button>
                ))}
              </div>
            </div>

            {/* Step 1: Callback Info */}
            <div className="rounded-lg border border-border/80 bg-muted/40 p-3.5 text-xs space-y-2">
              <div className="flex items-center justify-between font-semibold text-foreground">
                <span>Step 1: Application Callback Registration</span>
                {provider === 'github' && (
                  <a
                    href="https://github.com/settings/applications/new"
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 text-primary hover:underline"
                  >
                    GitHub Dev Settings <ExternalLink className="size-3" />
                  </a>
                )}
                {provider === 'google' && (
                  <a
                    href="https://console.cloud.google.com/apis/credentials"
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 text-primary hover:underline"
                  >
                    Google Cloud Console <ExternalLink className="size-3" />
                  </a>
                )}
              </div>

              <div className="space-y-1.5">
                <div>
                  <span className="text-muted-foreground">App Host URL: </span>
                  <Input
                    value={customAppUrl}
                    onChange={(e) => setCustomAppUrl(e.target.value)}
                    className="mt-1 h-8 text-xs font-mono"
                    placeholder="http://localhost:8080"
                  />
                </div>
                <div className="pt-1">
                  <span className="text-muted-foreground">Redirect / Callback URL: </span>
                  <div className="flex items-center justify-between gap-2 mt-0.5">
                    <code className="flex-1 rounded bg-background px-2 py-1 text-emerald-400 font-mono text-[11px] truncate border border-border/40">
                      {currentCallbackUrl}
                    </code>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={copyCallback}
                      className="size-7 p-0 shrink-0"
                    >
                      {copied ? <Check className="size-3.5 text-emerald-400" /> : <Copy className="size-3.5" />}
                    </Button>
                  </div>
                </div>
              </div>
            </div>

            {/* Step 2: Provider Fields */}
            <div className="space-y-3">
              <div className="text-xs font-semibold text-foreground">Step 2: Enter Provider Credentials</div>

              {provider === 'github' && (
                <>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">GitHub Client ID *</label>
                    <Input
                      placeholder="e.g. Ov23li..."
                      value={ghClientId}
                      onChange={(e) => setGhClientId(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">GitHub Client Secret *</label>
                    <Input
                      type="password"
                      placeholder="e.g. 43a886f4..."
                      value={ghClientSecret}
                      onChange={(e) => setGhClientSecret(e.target.value)}
                      required
                    />
                  </div>
                </>
              )}

              {provider === 'google' && (
                <>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">Google Client ID *</label>
                    <Input
                      placeholder="xxxx.apps.googleusercontent.com"
                      value={googleClientId}
                      onChange={(e) => setGoogleClientId(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">Google Client Secret *</label>
                    <Input
                      type="password"
                      value={googleClientSecret}
                      onChange={(e) => setGoogleClientSecret(e.target.value)}
                      required
                    />
                  </div>
                </>
              )}

              {provider === 'oidc' && (
                <>
                  <div className="grid grid-cols-2 gap-2">
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Provider Display Name</label>
                      <Input value={oidcName} onChange={(e) => setOidcName(e.target.value)} />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Client ID *</label>
                      <Input value={oidcClientId} onChange={(e) => setOidcClientId(e.target.value)} required />
                    </div>
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">Client Secret *</label>
                    <Input
                      type="password"
                      value={oidcClientSecret}
                      onChange={(e) => setOidcClientSecret(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">Authorization Endpoint URL *</label>
                    <Input
                      placeholder="https://auth.example.com/oauth/authorize"
                      value={oidcAuthUrl}
                      onChange={(e) => setOidcAuthUrl(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">Token Endpoint URL *</label>
                    <Input
                      placeholder="https://auth.example.com/oauth/token"
                      value={oidcTokenUrl}
                      onChange={(e) => setOidcTokenUrl(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">UserInfo Endpoint URL *</label>
                    <Input
                      placeholder="https://auth.example.com/oauth/userinfo"
                      value={oidcUserinfoUrl}
                      onChange={(e) => setOidcUserinfoUrl(e.target.value)}
                      required
                    />
                  </div>
                </>
              )}
            </div>
          </CardContent>

          <CardFooter className="pt-2">
            <Button type="submit" variant="default" className="w-full font-semibold" disabled={loading}>
              {loading ? 'Saving & Redirecting...' : `Save & Authenticate via ${provider.toUpperCase()}`}
              <ArrowRight className="size-4 ml-1.5" />
            </Button>
          </CardFooter>
        </form>
      </Card>
    </div>
  )
}
