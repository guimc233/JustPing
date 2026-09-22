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
  Route,
  Globe,
  X,
} from 'lucide-react'

interface AdminPanelProps {
  currentUser: any
}

interface ProxyProbe {
  id: string
  name: string
}

interface ProxyCredential {
  id: string
  agent_id: string
  agent_name: string
  username: string
  expires_at: string
  ttl_sec: number
  password?: string
  proxy_url?: string
}

function parseProxyCredential(value: unknown): ProxyCredential | null {
  if (typeof value !== 'object' || value === null) return null
  if (
    !('id' in value) ||
    !('agent_id' in value) ||
    !('agent_name' in value) ||
    !('username' in value) ||
    !('expires_at' in value) ||
    !('ttl_sec' in value)
  ) {
    return null
  }
  const { id, agent_id, agent_name, username, expires_at, ttl_sec } = value
  if (
    typeof id !== 'string' ||
    typeof agent_id !== 'string' ||
    typeof agent_name !== 'string' ||
    typeof username !== 'string' ||
    typeof expires_at !== 'string' ||
    typeof ttl_sec !== 'number'
  ) {
    return null
  }
  const credential: ProxyCredential = {
    id,
    agent_id,
    agent_name,
    username,
    expires_at,
    ttl_sec,
  }
  if ('password' in value && typeof value.password === 'string') credential.password = value.password
  if ('proxy_url' in value && typeof value.proxy_url === 'string') credential.proxy_url = value.proxy_url
  return credential
}

function parseProxyCredentialList(value: unknown): ProxyCredential[] {
  if (!Array.isArray(value)) return []
  const credentials: ProxyCredential[] = []
  for (const item of value) {
    const credential = parseProxyCredential(item)
    if (credential) credentials.push(credential)
  }
  return credentials
}

async function readProxyError(res: Response, fallback: string): Promise<string> {
  const body: unknown = await res.json().catch(() => null)
  if (typeof body === 'object' && body !== null && 'error' in body && typeof body.error === 'string' && body.error !== '') {
    return body.error
  }
  return fallback
}

export const AdminPanel: React.FC<AdminPanelProps> = ({ currentUser }) => {
  const [subTab, setSubTab] = useState<'targets' | 'agents' | 'whitelist' | 'settings'>('targets')

  // Targets state
  const [targets, setTargets] = useState<any[]>([])
  const [showAddTarget, setShowAddTarget] = useState(false)
  const [newTarget, setNewTarget] = useState({
    name: '',
    host: '',
    packet_count: 20,
    interval_sec: 30,
    tags: '',
    disable_route: false,
  })

  // Agents state
  const [agents, setAgents] = useState<any[]>([])
  const [showAddAgent, setShowAddAgent] = useState(false)
  const [newAgentName, setNewAgentName] = useState('')
  const [enrollResult, setEnrollResult] = useState<any>(null)
  const [copiedCmd, setCopiedCmd] = useState(false)
  const [editingAgentRoutes, setEditingAgentRoutes] = useState<any>(null)
  const [agentRouteDisabledMap, setAgentRouteDisabledMap] = useState<{ [targetId: string]: boolean }>({})
  const [proxyAgent, setProxyAgent] = useState<ProxyProbe | null>(null)
  const [proxyIssued, setProxyIssued] = useState<ProxyCredential | null>(null)
  const [proxySessions, setProxySessions] = useState<ProxyCredential[]>([])
  const [proxyError, setProxyError] = useState('')
  const [proxyBusy, setProxyBusy] = useState(false)
  const [copiedProxy, setCopiedProxy] = useState('')

  const loadProxySessions = async () => {
    const res = await fetch('/api/admin/proxy/sessions')
    if (!res.ok) return
    setProxySessions(parseProxyCredentialList(await res.json()))
  }

  const openProxyModal = async (agent: ProxyProbe) => {
    setProxyAgent(agent)
    setProxyIssued(null)
    setProxyError('')
    setCopiedProxy('')
    await loadProxySessions()
  }

  const issueProxy = async () => {
    if (!proxyAgent) return
    setProxyBusy(true)
    setProxyError('')
    const res = await fetch('/api/admin/proxy/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ agent_id: proxyAgent.id }),
    })
    setProxyBusy(false)
    if (res.ok) {
      const issued = parseProxyCredential(await res.json())
      if (!issued) {
        setProxyError('Failed to read proxy credential')
        return
      }
      setProxyIssued(issued)
      loadProxySessions()
    } else {
      setProxyError(await readProxyError(res, 'Failed to issue proxy credential'))
    }
  }

  const revokeProxy = async (id: string) => {
    const res = await fetch(`/api/admin/proxy/sessions/${id}`, { method: 'DELETE' })
    if (!res.ok) return
    if (proxyIssued?.id === id) setProxyIssued(null)
    loadProxySessions()
  }

  const copyProxyValue = async (label: string, value: string) => {
    await navigator.clipboard.writeText(value)
    setCopiedProxy(label)
    setTimeout(() => setCopiedProxy(''), 2000)
  }

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
      setNewTarget({ name: '', host: '', packet_count: 20, interval_sec: 30, tags: '', disable_route: false })
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

  const handleToggleTargetRoute = async (t: any) => {
    const res = await fetch(`/api/admin/targets/${t.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ disable_route: !t.disable_route }),
    })
    if (res.ok) {
      loadTargets()
    } else {
      const err = await res.json().catch(() => ({}))
      alert(err.error || 'Failed to update target route status')
    }
  }

  const handleOpenAgentRouteModal = (agent: any) => {
    const map: { [targetId: string]: boolean } = {}
    if (agent.disabled_route_targets) {
      agent.disabled_route_targets.split(',').forEach((id: string) => {
        const trimmed = id.trim()
        if (trimmed) map[trimmed] = true
      })
    }
    setAgentRouteDisabledMap(map)
    setEditingAgentRoutes(agent)
  }

  const handleSaveAgentRouteOverrides = async () => {
    if (!editingAgentRoutes) return
    const disabledList = Object.keys(agentRouteDisabledMap).filter((id) => agentRouteDisabledMap[id])
    const res = await fetch(`/api/admin/agents/${editingAgentRoutes.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        disabled_route_targets: disabledList.join(','),
      }),
    })
    if (res.ok) {
      setEditingAgentRoutes(null)
      loadAgents()
    } else {
      const err = await res.json().catch(() => ({}))
      alert(err.error || 'Failed to save route overrides')
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
                    <label className="text-xs text-muted-foreground">Sliding Window (samples)</label>
                    <Input
                      type="number"
                      min={3}
                      max={100}
                      value={newTarget.packet_count}
                      onChange={(e) => setNewTarget({ ...newTarget, packet_count: parseInt(e.target.value) || 20 })}
                    />
                  </div>
                  <div>
                    <label className="text-xs text-muted-foreground">Ping Interval (seconds)</label>
                    <Input
                      type="number"
                      min={5}
                      max={3600}
                      value={newTarget.interval_sec}
                      onChange={(e) => setNewTarget({ ...newTarget, interval_sec: parseInt(e.target.value) || 30 })}
                    />
                  </div>
                </div>
                <div className="flex items-center gap-2 pt-1">
                  <input
                    type="checkbox"
                    id="disableRouteCheck"
                    checked={newTarget.disable_route}
                    onChange={(e) => setNewTarget({ ...newTarget, disable_route: e.target.checked })}
                    className="rounded border-border text-primary focus:ring-primary size-3.5 cursor-pointer"
                  />
                  <label htmlFor="disableRouteCheck" className="text-xs text-muted-foreground cursor-pointer select-none">
                    Disable Route Trace (为此目标全局关闭路由检测)
                  </label>
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
                  <TableHead>Window</TableHead>
                  <TableHead>Interval</TableHead>
                  <TableHead>Route Trace</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {targets.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell className="font-semibold text-xs">{t.name}</TableCell>
                    <TableCell className="font-mono text-xs">{t.host}</TableCell>
                    <TableCell className="text-xs">{t.packet_count} samples</TableCell>
                    <TableCell className="text-xs">{t.interval_sec}s</TableCell>
                    <TableCell>
                      <button
                        type="button"
                        onClick={() => handleToggleTargetRoute(t)}
                        className={`inline-flex items-center gap-1 rounded px-2 py-0.5 text-[10px] font-medium border cursor-pointer transition ${
                          t.disable_route
                            ? 'bg-zinc-800 text-zinc-400 border-zinc-700 hover:text-zinc-200'
                            : 'bg-primary/10 text-primary border-primary/30 hover:bg-primary/20'
                        }`}
                        title={t.disable_route ? 'Click to Enable Route Trace' : 'Click to Disable Route Trace'}
                      >
                        <Route className="size-3" />
                        <span>{t.disable_route ? 'Disabled' : 'Active'}</span>
                      </button>
                    </TableCell>
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
                    <TableCell className="text-xs">{a.version || '—'}</TableCell>
                    <TableCell>
                      <Badge variant={a.is_online ? 'success' : 'secondary'}>
                        {a.is_online ? 'Online' : 'Offline'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right space-x-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleOpenAgentRouteModal(a)}
                        title="Route Trace Overrides (配置此节点对特定目标的路由检测)"
                        className="text-primary hover:text-primary"
                      >
                        <Route className="size-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => openProxyModal(a)}
                        title="签发 10 分钟 HTTPS 代理，流量从该探针出口"
                        disabled={!a.is_online}
                        className="text-sky-400 hover:text-sky-300 disabled:opacity-40"
                      >
                        <Globe className="size-3.5" />
                      </Button>
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

      {/* Agent Route Overrides Modal */}
      {editingAgentRoutes && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
          <div className="relative w-full max-w-lg rounded-xl border border-border bg-background p-6 shadow-2xl space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-3">
              <div className="flex items-center gap-2">
                <Route className="size-5 text-primary" />
                <h3 className="text-base font-bold">
                  Route Trace Overrides: <span className="text-primary">{editingAgentRoutes.name}</span>
                </h3>
              </div>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setEditingAgentRoutes(null)}
                className="size-7"
              >
                <X className="size-4" />
              </Button>
            </div>

            <p className="text-xs text-muted-foreground">
              勾选以下目标以<strong>关闭</strong>该探针节点对其的路由追踪检测。未勾选的目标将保持正常的 12 小时与突变触发检测。
            </p>

            <div className="max-h-60 overflow-y-auto space-y-2 border border-border/80 rounded-lg p-3 bg-muted/20">
              {targets.length === 0 ? (
                <div className="text-xs text-muted-foreground text-center py-4">暂无配置的目标</div>
              ) : (
                targets.map((tg) => {
                  const isDisabled = !!agentRouteDisabledMap[tg.id]
                  return (
                    <label
                      key={tg.id}
                      className={`flex items-center justify-between p-2 rounded-md border text-xs cursor-pointer transition ${
                        isDisabled
                          ? 'border-amber-500/40 bg-amber-500/10 text-amber-200'
                          : 'border-border bg-card/60 text-foreground hover:bg-muted/40'
                      }`}
                    >
                      <div className="space-y-0.5">
                        <div className="font-semibold">{tg.name}</div>
                        <div className="text-[10px] text-muted-foreground font-mono">{tg.host}</div>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className="text-[10px]">
                          {isDisabled ? (
                            <span className="text-amber-400 font-medium">Route 已关闭</span>
                          ) : (
                            <span className="text-emerald-400">Route 正常开启</span>
                          )}
                        </span>
                        <input
                          type="checkbox"
                          checked={isDisabled}
                          onChange={(e) => {
                            setAgentRouteDisabledMap({
                              ...agentRouteDisabledMap,
                              [tg.id]: e.target.checked,
                            })
                          }}
                          className="size-4 rounded border-border text-primary focus:ring-primary cursor-pointer"
                        />
                      </div>
                    </label>
                  )
                })
              )}
            </div>

            <div className="flex justify-end gap-2 pt-2 border-t border-border/60">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setEditingAgentRoutes(null)}
              >
                Cancel
              </Button>
              <Button size="sm" onClick={handleSaveAgentRouteOverrides}>
                Save Overrides
              </Button>
            </div>
          </div>
        </div>
      )}

      {proxyAgent && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
          <div className="relative w-full max-w-lg rounded-xl border border-border bg-background p-6 shadow-2xl space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-3">
              <div className="flex items-center gap-2">
                <Globe className="size-5 text-sky-400" />
                <h3 className="text-base font-bold">
                  HTTPS Proxy: <span className="text-primary">{proxyAgent.name}</span>
                </h3>
              </div>
              <Button variant="ghost" size="icon" onClick={() => setProxyAgent(null)} className="size-7">
                <X className="size-4" />
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              登录管理端后签发账号密码，有效期 10 分钟。把地址设为 <code>https_proxy</code> 后，HTTPS
              CONNECT 从这台探针出口。密码只显示这一次。探针需升级到支持代理转发的版本。
            </p>
            <Button size="sm" onClick={issueProxy} disabled={proxyBusy}>
              {proxyBusy ? '签发中…' : '签发 10 分钟账号'}
            </Button>
            {proxyError && <p className="text-xs text-destructive">{proxyError}</p>}
            {proxyIssued?.password && proxyIssued.proxy_url && (
              <div className="space-y-2 rounded-lg border border-primary/30 bg-muted/20 p-3">
                <div className="text-[11px] text-muted-foreground">到期 {proxyIssued.expires_at}</div>
                <div className="font-mono text-xs">用户名 {proxyIssued.username}</div>
                <div className="font-mono text-xs break-all">密码 {proxyIssued.password}</div>
                <div className="relative">
                  <pre className="overflow-x-auto rounded bg-zinc-950 p-2.5 pr-10 font-mono text-[11px] text-emerald-400">
                    {proxyIssued.proxy_url}
                  </pre>
                  <Button
                    variant="outline"
                    size="sm"
                    className="absolute right-2 top-2 size-7 p-0"
                    onClick={() => copyProxyValue('url', proxyIssued.proxy_url || '')}
                  >
                    {copiedProxy === 'url' ? <Check className="size-3.5 text-emerald-400" /> : <Copy className="size-3.5" />}
                  </Button>
                </div>
              </div>
            )}
            <div className="space-y-2">
              <div className="text-xs font-semibold">有效凭证</div>
              {proxySessions.filter((item) => item.agent_id === proxyAgent.id).length === 0 ? (
                <div className="text-xs text-muted-foreground">没有未过期的凭证</div>
              ) : (
                proxySessions
                  .filter((item) => item.agent_id === proxyAgent.id)
                  .map((item) => (
                    <div key={item.id} className="flex items-center justify-between gap-2 text-xs">
                      <span className="font-mono">{item.username}</span>
                      <span className="text-muted-foreground">{item.expires_at}</span>
                      <Button variant="ghost" size="sm" onClick={() => revokeProxy(item.id)}>
                        吊销
                      </Button>
                    </div>
                  ))
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
