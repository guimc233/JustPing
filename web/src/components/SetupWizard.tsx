import React, { useState } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from './ui/card'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Badge } from './ui/badge'
import { KeyRound, ShieldAlert, ArrowRight, ExternalLink, Copy, Check } from 'lucide-react'

interface SetupWizardProps {
  appUrl: string
  callbackUrl: string
  onComplete: () => void
}

export const SetupWizard: React.FC<SetupWizardProps> = ({ appUrl, callbackUrl }) => {
  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [customAppUrl, setCustomAppUrl] = useState(appUrl || window.location.origin)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const copyCallback = () => {
    navigator.clipboard.writeText(callbackUrl)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!clientId.trim() || !clientSecret.trim()) {
      setError('Please provide both GitHub Client ID and Client Secret.')
      return
    }

    setLoading(true)
    setError(null)

    try {
      const res = await fetch('/api/setup/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          github_client_id: clientId.trim(),
          github_client_secret: clientSecret.trim(),
          app_url: customAppUrl.trim(),
        }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Failed to save configuration')
      }

      // Redirect to OAuth login with setup state
      window.location.href = data.redirect_url || '/api/auth/github/login?state=setup'
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
            Initial System Setup Required
          </Badge>
          <CardTitle className="text-2xl">Welcome to JustPing</CardTitle>
          <CardDescription>
            Configure GitHub OAuth to initialize your platform. The first user who completes this authentication will become the <strong>Superadmin</strong>.
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

            {/* Step 1 Guide */}
            <div className="rounded-lg border border-border/80 bg-muted/40 p-4 text-xs space-y-2">
              <div className="flex items-center justify-between font-semibold text-foreground">
                <span>Step 1: Create a GitHub OAuth App</span>
                <a
                  href="https://github.com/settings/applications/new"
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 text-primary hover:underline"
                >
                  GitHub Developer Settings <ExternalLink className="size-3" />
                </a>
              </div>
              <div className="text-muted-foreground">
                Fill in Application Name (e.g. <code>JustPing Monitor</code>) and:
              </div>
              <div className="space-y-1">
                <div>
                  <span className="text-muted-foreground">Homepage URL: </span>
                  <code className="rounded bg-background px-1.5 py-0.5 text-foreground">{customAppUrl}</code>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <div className="truncate">
                    <span className="text-muted-foreground">Authorization callback URL: </span>
                    <code className="rounded bg-background px-1.5 py-0.5 text-emerald-400 font-mono text-[11px] truncate">
                      {callbackUrl}
                    </code>
                  </div>
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

            {/* Step 2 Inputs */}
            <div className="space-y-3">
              <div className="text-xs font-semibold text-foreground">
                Step 2: Enter Credentials
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">
                  GitHub Client ID <span className="text-destructive">*</span>
                </label>
                <Input
                  placeholder="e.g. Ov23li..."
                  value={clientId}
                  onChange={(e) => setClientId(e.target.value)}
                  required
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">
                  GitHub Client Secret <span className="text-destructive">*</span>
                </label>
                <Input
                  type="password"
                  placeholder="e.g. 43a886f4..."
                  value={clientSecret}
                  onChange={(e) => setClientSecret(e.target.value)}
                  required
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">
                  App Host URL
                </label>
                <Input
                  value={customAppUrl}
                  onChange={(e) => setCustomAppUrl(e.target.value)}
                  placeholder="http://localhost:8080"
                />
              </div>
            </div>
          </CardContent>

          <CardFooter className="pt-2">
            <Button
              type="submit"
              variant="default"
              className="w-full font-semibold"
              disabled={loading}
            >
              {loading ? 'Saving & Redirecting...' : 'Save & Authenticate as Superadmin'}
              <ArrowRight className="size-4 ml-1.5" />
            </Button>
          </CardFooter>
        </form>
      </Card>
    </div>
  )
}
