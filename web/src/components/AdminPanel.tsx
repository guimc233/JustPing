import React, { useState, useEffect } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from './ui/card'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Badge } from './ui/badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from './ui/table'
import {
  Target,
  Server,
  Mail,
  Settings,
  Plus,
  Trash2,
  Copy,
  Check,
  Power,
  Terminal,
  ShieldCheck,
  KeyRound,
} from 'lucide-react'

interface AdminPanelProps {
  currentUser: any
}

export const AdminPanel: React.FC<AdminPanelProps> = ({ currentUser }) => {
  const [subTab, setSubTab] = useState<'targets' | 'agents' | 'whitelist' | 'settings'>('targets')

  // Targets state
  const [targets, setTargets] = useState<any[]>([])
  const [showAddTarget, setShowAddTarget] = useState(false)
  const [newTarget, setNewTarget] = useState({
    name: '',
    host: '',
    packet_count: 15,
    interval_sec: 60,
    tags: '',
  })

  // Agents state
  const [agents, setAgents] = useState<any[]>([])
  const [showAddAgent, setShowAddAgent] = useState(false)
  const [newAgentName, setNewAgentName] = useState('')
  const [enrollResult, setEnrollResult] = useState<any>(null)
  const [copiedCmd, setCopiedCmd] = useState(false)

  // Whitelist state
  const [whitelist, setWhitelist] = useState<any[]>([])
  const [newEmail, setNewEmail] = useState('')
  const [newEmailRemark, setNewEmailRemark] = useState('')

  // Settings state
  const [settings, setSettings] = useState<any>({})
  const [saveSettingsSuccess, setSaveSettingsSuccess] = useState(false)

  const isSuperadmin = currentUser?.user?.role === 'superadmin'

  const loadTargets = async () => {
    const res = await fetch('/api/admin/targets')
    if (res.ok) setTargets(await res.json())
  }

  const loadAgents = async () => {
    const res = await fetch('/api/admin/agents')
    if (res.ok) setAgents(await res.json())
  }

  const loadWhitelist = async () => {
    const res = await fetch('/api/admin/whitelist')
    if (res.ok) setWhitelist(await res.json())
  }

  const loadSettings = async () => {
    if (!isSuperadmin) return
    const res = await fetch('/api/admin/settings')
    if (res.ok) setSettings(await res.json())
  }

  useEffect(() => {
    loadTargets()
    loadAgents()
    loadWhitelist()
    if (isSuperadmin) loadSettings()
  }, [])

  // Target handlers
  const handleAddTarget = async (e: React.FormEvent) => {
    e.preventDefault()
    const res = await fetch('/api/admin/targets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newTarget),
    })
    if (res.ok) {
      setNewTarget({ name: '', host: '', packet_count: 15, interval_sec: 60, tags: '' })
      setShowAddTarget(false)
      loadTargets()
    } else {
      const err = await res.json().catch(() => ({}))
      alert(err.error || 'Failed to save target')
    }
  }

  const handleDeleteTarget = async (id: string) => {
    if (!confirm('Are you sure you want to delete this target?')) return
    const res = await fetch(`/api/admin/targets/${id}`, { method: 'DELETE' })
    if (res.ok) {
      loadTargets()
    } else {
      const err = await res.json().catch(() => ({}))
      alert(err.error || 'Failed to delete target')
    }
  }

  const handleToggleTarget = async (t: any) => {
    const res = await fetch(`/api/admin/targets/${t.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: !t.enabled }),
    })
    if (res.ok) {
      loadTargets()
    } else {
      const err = await res.json().catch(() => ({}))
      alert(err.error || 'Failed to update target status')
    }
  }

  // Agent handlers
  const handleCreateAgent = async (e: React.FormEvent) => {
    e.preventDefault()
    const res = await fetch('/api/admin/agents', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: newAgentName.trim() }),
    })
    if (res.ok) {
      const data = await res.json()
      setEnrollResult(data)
      setNewAgentName('')
      loadAgents()
    }
  }

  const handleRotateToken = async (id: string, name: string) => {
    if (!confirm(`Rotate enrollment token for probe "${name}"? Active connection will be closed.`)) return
    const res = await fetch(`/api/admin/agents/${id}/rotate-token`, { method: 'POST' })
    if (res.ok) {
      const data = await res.json()
      setEnrollResult(data)
      setShowAddAgent(true)
      loadAgents()
    } else {
      const err = await res.json().catch(() => ({}))
      alert(err.error || 'Failed to rotate token')
    }
  }

  const handleDeleteAgent = async (id: string) => {
    if (!confirm('Are you sure you want to remove this probe?')) return
    await fetch(`/api/admin/agents/${id}`, { method: 'DELETE' })
    loadAgents()
  }

  // Whitelist handlers
  const handleAddWhitelist = async (e: React.FormEvent) => {
    e.preventDefault()
    const res = await fetch('/api/admin/whitelist', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: newEmail.trim(), remark: newEmailRemark.trim() }),
    })
    if (res.ok) {
      setNewEmail('')
      setNewEmailRemark('')
      loadWhitelist()
    } else {
      const err = await res.json()
      alert(err.error || 'Failed to add whitelist email')
    }
  }

  const handleDeleteWhitelist = async (id: number) => {
    if (!confirm('Remove this email from trusted whitelist?')) return
    const res = await fetch(`/api/admin/whitelist/${id}`, { method: 'DELETE' })
    if (res.ok) {
      loadWhitelist()
    } else {
      const err = await res.json()
      alert(err.error || 'Cannot remove')
    }
  }

  // Settings handlers
  const handleSaveSettings = async (e: React.FormEvent) => {
    e.preventDefault()
    const res = await fetch('/api/admin/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings),
    })
    if (res.ok) {
      setSaveSettingsSuccess(true)
      setTimeout(() => setSaveSettingsSuccess(false), 3000)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      {/* Sub Tabs */}
      <div className="flex items-center gap-2 border-b border-border pb-3">
        <Button
          variant={subTab === 'targets' ? 'default' : 'outline'}
          size="sm"
          onClick={() => setSubTab('targets')}
        >
          <Target className="size-4 mr-1.5" />
          Ping Targets ({targets.length})
        </Button>
        <Button
          variant={subTab === 'agents' ? 'default' : 'outline'}
          size="sm"
          onClick={() => setSubTab('agents')}
        >
          <Server className="size-4 mr-1.5" />
          Probes / Agents ({agents.length})
        </Button>
        <Button
          variant={subTab === 'whitelist' ? 'default' : 'outline'}
          size="sm"
          onClick={() => setSubTab('whitelist')}
        >
          <Mail className="size-4 mr-1.5" />
          Email Whitelist ({whitelist.length})
        </Button>
        {isSuperadmin && (
          <Button
            variant={subTab === 'settings' ? 'default' : 'outline'}
            size="sm"
            onClick={() => setSubTab('settings')}
          >
            <Settings className="size-4 mr-1.5" />
            System Settings
          </Button>
        )}
      </div>

      {/* 1. Targets SubTab */}
      {subTab === 'targets' && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle>ICMP Ping Targets</CardTitle>
              <CardDescription>
                Define hosts or IPs to be monitored periodically by distributed agents.
              </CardDescription>
            </div>
            <Button size="sm" onClick={() => setShowAddTarget(!showAddTarget)}>
              <Plus className="size-4 mr-1" />
              Add Target
            </Button>
          </CardHeader>
          <CardContent className="space-y-4">
            {showAddTarget && (
              <form
                onSubmit={handleAddTarget}
                className="rounded-lg border border-border bg-muted/30 p-4 space-y-3"
              >
                <div className="text-xs font-semibold">New Ping Target</div>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div>
                    <label className="text-xs text-muted-foreground">Target Name</label>
                    <Input
                      placeholder="e.g. Cloudflare DNS"
                      value={newTarget.name}
                      onChange={(e) => setNewTarget({ ...newTarget, name: e.target.value })}
                      required
                    />
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">Host / IP</label>
                    <Input
                      placeholder="e.g. 1.1.1.1 or google.com"
                      value={newTarget.host}
                      onChange={(e) => setNewTarget({ ...newTarget, host: e.target.value })}
                      required
                    />
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">Packet Count (per round)</label>
                    <Input
                      type="number"
                      min={3}
                      max={50}
                      value={newTarget.packet_count}
                      onChange={(e) => setNewTarget({ ...newTarget, packet_count: parseInt(e.target.value) || 15 })}
                    />
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">Interval (seconds)</label>
                    <Input
                      type="number"
                      min={10}
                      max={3600}
                      value={newTarget.interval_sec}
                      onChange={(e) => setNewTarget({ ...newTarget, interval_sec: parseInt(e.target.value) || 60 })}
                    />
                  </div>
                </div>
                <div className="flex justify-end gap-2 pt-2">
                  <Button type="button" variant="ghost" size="sm" onClick={() => setShowAddTarget(false)}>
                    Cancel
                  </Button>
                  <Button type="submit" size="sm">
                    Save Target
                  </Button>
                </div>
              </form>
            )}

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Target</TableHead>
                  <TableHead>Host / IP</TableHead>
                  <TableHead>Packets</TableHead>
                  <TableHead>Interval</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {targets.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell className="font-semibold text-xs">{t.name}</TableCell>
                    <TableCell className="font-mono text-xs">{t.host}</TableCell>
                    <TableCell className="text-xs">{t.packet_count} pkts</TableCell>
                    <TableCell className="text-xs">{t.interval_sec}s</TableCell>
                    <TableCell>
                      <Badge variant={t.enabled ? 'success' : 'secondary'}>
                        {t.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right space-x-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleToggleTarget(t)}
                        title={t.enabled ? 'Disable' : 'Enable'}
                      >
                        <Power className={`size-3.5 ${t.enabled ? 'text-emerald-400' : 'text-zinc-500'}`} />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleDeleteTarget(t.id)}
                        className="text-destructive hover:text-destructive"
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {/* 2. Agents SubTab */}
      {subTab === 'agents' && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle>Distributed Probes / Agents</CardTitle>
              <CardDescription>
                Manage deployed Go agent probes. Admin view reveals unmasked remote IP addresses.
              </CardDescription>
            </div>
            <Button size="sm" onClick={() => setShowAddAgent(!showAddAgent)}>
              <Plus className="size-4 mr-1" />
              Enroll New Probe
            </Button>
          </CardHeader>
          <CardContent className="space-y-4">
            {showAddAgent && (
              <div className="rounded-lg border border-border bg-muted/30 p-4 space-y-3">
                <div className="text-xs font-semibold">Enroll Probe</div>
                <form onSubmit={handleCreateAgent} className="flex gap-2">
                  <Input
                    placeholder="Probe Name (e.g. Tokyo AWS Node)"
                    value={newAgentName}
                    onChange={(e) => setNewAgentName(e.target.value)}
                    required
                  />
                  <Button type="submit" size="sm">
                    Generate Installation Script
                  </Button>
                </form>

                {enrollResult && (
                  <div className="mt-3 rounded-lg border border-primary/30 bg-background p-3 space-y-2">
                    <div className="flex items-center gap-1.5 text-xs font-semibold text-primary">
                      <Terminal className="size-4" />
                      <span>One-Line Auto Installation (systemd / OpenRC / procd / runit / sysvinit)</span>
                    </div>
                    <div className="relative flex items-center">
                      <pre className="w-full overflow-x-auto rounded bg-zinc-950 p-2.5 font-mono text-[11px] text-emerald-400">
                        {enrollResult.install_command}
                      </pre>
                      <Button
                        variant="outline"
                        size="sm"
                        className="absolute right-2 top-2 size-7 p-0"
                        onClick={() => {
                          navigator.clipboard.writeText(enrollResult.install_command)
                          setCopiedCmd(true)
                          setTimeout(() => setCopiedCmd(false), 2000)
                        }}
                      >
                        {copiedCmd ? <Check className="size-3.5 text-emerald-400" /> : <Copy className="size-3.5" />}
                      </Button>
                    </div>
                    <div className="text-[11px] text-muted-foreground">
                      Run this command with <code>sudo</code> on any Linux probe. It automatically detects init
                      system and architecture, downloads the matching binary, installs as a service and connects back to this Host.
                    </div>
                  </div>
                )}
              </div>
            )}

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Probe</TableHead>
                  <TableHead>Public IP</TableHead>
                  <TableHead>OS / Arch</TableHead>
                  <TableHead>Version</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {agents.map((a) => (
                  <TableRow key={a.id}>
                    <TableCell className="font-semibold text-xs">{a.name}</TableCell>
                    <TableCell className="font-mono text-xs">{a.public_ip || '—'}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {a.os || '—'} / {a.arch || '—'}
                    </TableCell>
                    <TableCell className="text-xs">{a.version || '1.0.0'}</TableCell>
                    <TableCell>
                      <Badge variant={a.is_online ? 'success' : 'secondary'}>
                        {a.is_online ? 'Online' : 'Offline'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right space-x-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleRotateToken(a.id, a.name)}
                        title="Rotate Token"
                        className="text-amber-400 hover:text-amber-300"
                      >
                        <KeyRound className="size-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleDeleteAgent(a.id)}
                        className="text-destructive hover:text-destructive"
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {/* 3. Whitelist SubTab */}
      {subTab === 'whitelist' && (
        <Card>
          <CardHeader>
            <CardTitle>Trusted Email Whitelist</CardTitle>
            <CardDescription>
              Users whose verified OAuth email (GitHub, Google, or OIDC) is in this list are allowed to log into the Admin Panel.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <form onSubmit={handleAddWhitelist} className="flex flex-col sm:flex-row gap-2">
              <Input
                type="email"
                placeholder="Verified email address (e.g. user@example.com)"
                value={newEmail}
                onChange={(e) => setNewEmail(e.target.value)}
                required
              />
              <Input
                placeholder="Remark (optional, e.g. DevOps Engineer)"
                value={newEmailRemark}
                onChange={(e) => setNewEmailRemark(e.target.value)}
              />
              <Button type="submit" size="sm" className="shrink-0">
                <Plus className="size-4 mr-1" />
                Add to Whitelist
              </Button>
            </form>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Email</TableHead>
                  <TableHead>Remark</TableHead>
                  <TableHead>Added By</TableHead>
                  <TableHead>Date</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {whitelist.map((w) => (
                  <TableRow key={w.id}>
                    <TableCell className="font-semibold text-xs">{w.email}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">{w.remark || '—'}</TableCell>
                    <TableCell className="text-xs">{w.created_by || 'system'}</TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {new Date(w.created_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleDeleteWhitelist(w.id)}
                        className="text-destructive hover:text-destructive"
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      {/* 4. Settings SubTab (Superadmin) */}
      {subTab === 'settings' && isSuperadmin && (
        <Card>
          <CardHeader>
            <CardTitle>System & OAuth Settings</CardTitle>
            <CardDescription>
              Manage OAuth credentials (GitHub, Google, OIDC) and data retention rules.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSaveSettings} className="max-w-xl space-y-4">
              {saveSettingsSuccess && (
                <div className="flex items-center gap-2 rounded-lg bg-emerald-500/15 p-3 text-xs text-emerald-400 border border-emerald-500/20">
                  <ShieldCheck className="size-4" />
                  <span>Settings updated successfully.</span>
                </div>
              )}

              <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">GitHub Client ID</label>
                <Input
                  value={settings.github_client_id || ''}
                  onChange={(e) => setSettings({ ...settings, github_client_id: e.target.value })}
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">GitHub Client Secret</label>
                <Input
                  type="password"
                  placeholder="Keep unchanged or enter new secret"
                  value={settings.github_client_secret || ''}
                  onChange={(e) => setSettings({ ...settings, github_client_secret: e.target.value })}
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-medium text-muted-foreground">Application Base URL</label>
                <Input
                  value={settings.app_url || ''}
                  onChange={(e) => setSettings({ ...settings, app_url: e.target.value })}
                />
              </div>

              {/* Google OAuth */}
              <div className="pt-2 border-t border-border/60">
                <div className="text-xs font-semibold text-foreground mb-2">Google OAuth Credentials</div>
                <div className="space-y-2">
                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">Google Client ID</label>
                    <Input
                      value={settings.google_client_id || ''}
                      onChange={(e) => setSettings({ ...settings, google_client_id: e.target.value })}
                      placeholder="xxxx.apps.googleusercontent.com"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">Google Client Secret</label>
                    <Input
                      type="password"
                      placeholder="Keep unchanged or enter new secret"
                      value={settings.google_client_secret || ''}
                      onChange={(e) => setSettings({ ...settings, google_client_secret: e.target.value })}
                    />
                  </div>
                </div>
              </div>

              {/* Generic OIDC */}
              <div className="pt-2 border-t border-border/60">
                <div className="text-xs font-semibold text-foreground mb-2">Generic OIDC / Custom OAuth</div>
                <div className="space-y-2">
                  <div className="grid grid-cols-2 gap-2">
                    <div className="space-y-1">
                      <label className="text-xs font-medium text-muted-foreground">Provider Display Name</label>
                      <Input
                        value={settings.oidc_name || ''}
                        onChange={(e) => setSettings({ ...settings, oidc_name: e.target.value })}
                        placeholder="SSO / Keycloak"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs font-medium text-muted-foreground">OIDC Client ID</label>
                      <Input
                        value={settings.oidc_client_id || ''}
                        onChange={(e) => setSettings({ ...settings, oidc_client_id: e.target.value })}
                      />
                    </div>
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">OIDC Client Secret</label>
                    <Input
                      type="password"
                      placeholder="Keep unchanged or enter new secret"
                      value={settings.oidc_client_secret || ''}
                      onChange={(e) => setSettings({ ...settings, oidc_client_secret: e.target.value })}
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">Auth Endpoint URL</label>
                    <Input
                      value={settings.oidc_auth_url || ''}
                      onChange={(e) => setSettings({ ...settings, oidc_auth_url: e.target.value })}
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">Token Endpoint URL</label>
                    <Input
                      value={settings.oidc_token_url || ''}
                      onChange={(e) => setSettings({ ...settings, oidc_token_url: e.target.value })}
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs font-medium text-muted-foreground">UserInfo Endpoint URL</label>
                    <Input
                      value={settings.oidc_userinfo_url || ''}
                      onChange={(e) => setSettings({ ...settings, oidc_userinfo_url: e.target.value })}
                    />
                  </div>
                </div>
              </div>

              <div className="space-y-1 pt-2 border-t border-border/60">
                <label className="text-xs font-medium text-muted-foreground">
                  Ping Metric Data Retention (Days)
                </label>
                <Input
                  type="number"
                  min={1}
                  max={365}
                  value={settings.retention_days || '30'}
                  onChange={(e) => setSettings({ ...settings, retention_days: e.target.value })}
                />
              </div>

              <Button type="submit">Save Changes</Button>
            </form>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
