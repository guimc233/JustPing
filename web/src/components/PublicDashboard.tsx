import React, { useState, useEffect, useRef } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from './ui/card'
import { Button } from './ui/button'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from './ui/table'
import { Badge } from './ui/badge'
import { Server, Zap, Shield, RefreshCw, ChevronRight, BarChart3, Wifi, Target, Activity, Route } from 'lucide-react'
import { SmokepingChart, type MetricDataPoint } from './SmokepingChart'
import { TracerouteDetail } from './TracerouteDetail'

interface PublicDashboardProps {
  isAdmin: boolean
}

export const PublicDashboard: React.FC<PublicDashboardProps> = ({ isAdmin }) => {
  const [summary, setSummary] = useState<any>(null)
  const [agents, setAgents] = useState<any[]>([])
  const [targets, setTargets] = useState<any[]>([])
  const [matrix, setMatrix] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  // Selected agent & target for deep-dive inspection
  const [selectedAgentId, setSelectedAgentId] = useState<string>('')
  const [selectedTargetId, setSelectedTargetId] = useState<string>('')
  const [chartRange, setChartRange] = useState<string>('1h')
  const [chartData, setChartData] = useState<MetricDataPoint[]>([])
  const [chartLoading, setChartLoading] = useState(false)
  const inspectCardRef = useRef<HTMLDivElement>(null)

  // Traceroute telemetry modal state
  const [tracerouteModalOpen, setTracerouteModalOpen] = useState(false)
  const [selectedTraceroute, setSelectedTraceroute] = useState<any>(null)
  const [tracerouteLoading, setTracerouteLoading] = useState(false)

  const handleChartTimeClick = async (timestampMs: number) => {
    if (!selectedAgentId) return
    console.log('[PublicDashboard] Chart timestamp clicked:', timestampMs, new Date(timestampMs).toLocaleString())
    const effTargetId = selectedTargetId || targets[0]?.id || ''
    setTracerouteLoading(true)
    setTracerouteModalOpen(true)
    try {
      let url = `/api/public/traceroute?agent_id=${selectedAgentId}&time=${timestampMs}`
      if (effTargetId) {
        url += `&target_id=${effTargetId}`
      }
      const res = await fetch(url)
      if (res.ok) {
        const data = await res.json()
        setSelectedTraceroute(data)
      } else {
        setSelectedTraceroute(null)
      }
    } catch (err) {
      console.error('Failed to query traceroute for timestamp:', err)
      setSelectedTraceroute(null)
    } finally {
      setTracerouteLoading(false)
    }
  }

  const handleOpenLatestTraceroute = async () => {
    if (!selectedAgentId) return
    const effTargetId = selectedTargetId || targets[0]?.id || ''
    setTracerouteLoading(true)
    setTracerouteModalOpen(true)
    try {
      let url = `/api/public/traceroute?agent_id=${selectedAgentId}`
      if (effTargetId) {
        url += `&target_id=${effTargetId}`
      }
      const res = await fetch(url)
      if (res.ok) {
        const data = await res.json()
        setSelectedTraceroute(data)
      } else {
        setSelectedTraceroute(null)
      }
    } catch (err) {
      console.error('Failed to query latest traceroute:', err)
      setSelectedTraceroute(null)
    } finally {
      setTracerouteLoading(false)
    }
  }

  const handleSelectTracerouteRecord = async (recordId: string) => {
    setTracerouteLoading(true)
    try {
      const res = await fetch(`/api/public/traceroute/${recordId}`)
      if (res.ok) {
        const data = await res.json()
        setSelectedTraceroute(data)
      }
    } catch (err) {
      console.error('Failed to load traceroute by ID:', err)
    } finally {
      setTracerouteLoading(false)
    }
  }

  const fetchData = async () => {
    try {
      const [sumRes, agRes, tgRes, mxRes] = await Promise.all([
        fetch('/api/public/summary'),
        fetch('/api/public/agents'),
        fetch('/api/public/targets'),
        fetch('/api/public/matrix'),
      ])

      const sumData = await sumRes.json()
      const agData = await agRes.json()
      const tgData = await tgRes.json()
      const mxData = await mxRes.json()

      setSummary(sumData)
      setAgents(agData)
      setTargets(tgData)
      setMatrix(mxData)

      setSelectedAgentId((prev) => {
        if (prev && agData.some((a: any) => a.id === prev)) {
          return prev
        }
        return agData.length > 0 ? agData[0].id : ''
      })

      setSelectedTargetId((prev) => {
        if (prev && tgData.some((t: any) => t.id === prev)) {
          return prev
        }
        return tgData.length > 0 ? tgData[0].id : ''
      })
    } catch (err) {
      console.error('Failed to load probe metrics:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchAgentChartMetrics = async () => {
    if (!selectedAgentId) return
    setChartLoading(true)
    try {
      let url = `/api/public/metrics?range=${chartRange}&agent_id=${selectedAgentId}`
      if (selectedTargetId) {
        url += `&target_id=${selectedTargetId}`
      }
      const res = await fetch(url)
      const data: MetricDataPoint[] = await res.json()
      setChartData(data)
    } catch (err) {
      console.error('Failed to load agent chart metrics:', err)
    } finally {
      setChartLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 15000)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    fetchAgentChartMetrics()
  }, [selectedAgentId, selectedTargetId, chartRange])

  const selectedAgent = agents.find((a) => a.id === selectedAgentId) || agents[0]
  const selectedTarget = targets.find((t) => t.id === selectedTargetId) || targets[0]

  // Calculate fleet health
  const onlineAgents = agents.filter((a) => a.is_online)
  const highQualityAgents = onlineAgents.filter(
    (a) => a.quality?.quality_grade === 'A+' || a.quality?.quality_grade === 'A'
  )
  const fleetHealthPct =
    onlineAgents.length > 0
      ? Math.round((highQualityAgents.length / onlineAgents.length) * 100)
      : 0

  const getMatrixCell = (agentId: string, targetId: string) => {
    return matrix.find((m) => m.agent_id === agentId && m.target_id === targetId)
  }

  const getGradeBadge = (grade: string) => {
    switch (grade) {
      case 'A+':
        return <Badge variant="success" className="font-bold">A+ Excellent</Badge>
      case 'A':
        return <Badge variant="success" className="font-bold">A Good</Badge>
      case 'B':
        return <Badge variant="secondary" className="font-bold text-blue-400">B Fair</Badge>
      case 'C':
        return <Badge variant="warning" className="font-bold">C Degraded</Badge>
      case 'F':
        return <Badge variant="destructive" className="font-bold">F Critical</Badge>
      default:
        return <Badge variant="outline">Offline</Badge>
    }
  }

  return (
    <div className="flex flex-col gap-6">
      {/* Privacy Notice Banner */}
      {!isAdmin && (
        <div className="flex items-center justify-between rounded-lg border border-border/80 bg-muted/30 px-4 py-2 text-xs text-muted-foreground">
          <div className="flex items-center gap-2">
            <Shield className="size-4 text-primary" />
            <span>
              Public view active: Probing nodes' IP addresses and sensitive network topology are masked.
            </span>
          </div>
          <Button variant="ghost" size="sm" onClick={fetchData} className="size-7 p-0">
            <RefreshCw className="size-3.5" />
          </Button>
        </div>
      )}

      {/* Fleet Summary Overview */}
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Card className="p-4 flex items-center justify-between">
          <div>
            <div className="text-xs font-medium text-muted-foreground">Online Probes</div>
            <div className="text-2xl font-bold mt-1">
              {summary?.online_agents || 0}
              <span className="text-xs font-normal text-muted-foreground ml-1">
                / {summary?.total_agents || 0}
              </span>
            </div>
          </div>
          <div className="size-10 rounded-lg bg-emerald-500/10 text-emerald-400 flex items-center justify-center">
            <Server className="size-5" />
          </div>
        </Card>

        <Card className="p-4 flex items-center justify-between">
          <div>
            <div className="text-xs font-medium text-muted-foreground">Probe Fleet Health</div>
            <div className="text-2xl font-bold mt-1 text-emerald-400">
              {fleetHealthPct}
              <span className="text-xs font-normal text-muted-foreground ml-1">%</span>
            </div>
          </div>
          <div className="size-10 rounded-lg bg-blue-500/10 text-blue-400 flex items-center justify-center">
            <Wifi className="size-5" />
          </div>
        </Card>

        <Card className="p-4 flex items-center justify-between">
          <div>
            <div className="text-xs font-medium text-muted-foreground">Probes Avg Latency</div>
            <div className="text-2xl font-bold mt-1">
              {summary?.avg_rtt_ms ? Number(summary.avg_rtt_ms).toFixed(1) : 0}{' '}
              <span className="text-xs font-normal text-muted-foreground">ms</span>
            </div>
          </div>
          <div className="size-10 rounded-lg bg-purple-500/10 text-purple-400 flex items-center justify-center">
            <BarChart3 className="size-5" />
          </div>
        </Card>

        <Card className="p-4 flex items-center justify-between">
          <div>
            <div className="text-xs font-medium text-muted-foreground">Probes Avg Loss</div>
            <div className="text-2xl font-bold mt-1">
              {summary?.avg_loss_pct ? Number(summary.avg_loss_pct).toFixed(1) : 0}{' '}
              <span className="text-xs font-normal text-muted-foreground">%</span>
            </div>
          </div>
          <div className="size-10 rounded-lg bg-amber-500/10 text-amber-400 flex items-center justify-center">
            <Zap className="size-5" />
          </div>
        </Card>
      </div>

      {/* Primary Section: Probe Network Quality Ranking */}
      <Card>
        <CardHeader className="flex flex-col sm:flex-row sm:items-center justify-between pb-2 gap-2">
          <div>
            <CardTitle className="text-lg">Probe Fleet Network Quality</CardTitle>
            <CardDescription className="text-xs">
              Live measurement of each probe node's latency stability, jitter, and packet loss against benchmark targets
            </CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="py-8 text-center text-sm text-muted-foreground">Loading probe network data...</div>
          ) : agents.length === 0 ? (
            <div className="py-8 text-center text-sm text-muted-foreground">
              No probes active. Install an agent on your server using the one-line installer in Admin Panel.
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {agents.map((agent) => {
                const isSelected = agent.id === selectedAgentId
                const q = agent.quality || {
                  avg_rtt_ms: 0,
                  jitter_ms: 0,
                  loss_pct: 0,
                  quality_score: 0,
                  quality_grade: agent.is_online ? 'A+' : 'Offline',
                }

                return (
                  <div
                    key={agent.id}
                    onClick={() => {
                      setSelectedAgentId(agent.id)
                      inspectCardRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
                    }}
                    className={`rounded-xl border p-4 transition-all cursor-pointer flex flex-col justify-between gap-3 ${
                      isSelected
                        ? 'border-primary bg-primary/5 shadow-md shadow-primary/10'
                        : 'border-border bg-card/60 hover:border-border/80 hover:bg-muted/30'
                    }`}
                  >
                    {/* Top Row: Name, Status & Grade */}
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span
                            className={`size-2 rounded-full shrink-0 ${
                              agent.is_online ? 'bg-emerald-400 animate-pulse' : 'bg-zinc-600'
                            }`}
                          />
                          <div className="font-semibold text-sm truncate">{agent.name}</div>
                        </div>
                        <div className="text-[11px] text-muted-foreground font-mono mt-0.5 truncate pl-4">
                          {agent.public_ip || 'unknown IP'} • {agent.os || 'linux'}/{agent.arch || 'amd64'}
                        </div>
                      </div>
                      {getGradeBadge(q.quality_grade)}
                    </div>

                    {/* Middle: Quality Metrics */}
                    <div className="grid grid-cols-3 gap-2 rounded-lg bg-muted/40 p-2 text-center text-xs">
                      <div>
                        <div className="text-[10px] text-muted-foreground">Avg Latency</div>
                        <div className="font-bold text-foreground mt-0.5">{q.avg_rtt_ms} ms</div>
                      </div>
                      <div>
                        <div className="text-[10px] text-muted-foreground">Jitter</div>
                        <div className="font-bold text-foreground mt-0.5">{q.jitter_ms} ms</div>
                      </div>
                      <div>
                        <div className="text-[10px] text-muted-foreground">Loss</div>
                        <div
                          className={`font-bold mt-0.5 ${
                            q.loss_pct > 0 ? 'text-destructive' : 'text-emerald-400'
                          }`}
                        >
                          {q.loss_pct}%
                        </div>
                      </div>
                    </div>

                    {/* Bottom: Action link */}
                    <div className="flex items-center justify-between text-[11px] text-muted-foreground pt-1 border-t border-border/40">
                      <span>Quality Score: <strong className="text-foreground">{q.quality_score}</strong>/100</span>
                      <span className="inline-flex items-center text-primary font-medium">
                        Inspect Network <ChevronRight className="size-3 ml-0.5" />
                      </span>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Secondary Section: SmokePing Network Stability Analysis */}
      {selectedAgent && (
        <Card ref={inspectCardRef} className="border-primary/40 scroll-mt-6">
          <CardHeader className="flex flex-col gap-3 pb-2">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
              <div>
                <div className="flex items-center gap-2">
                  <Activity className="size-4 text-primary" />
                  <CardTitle className="text-base">
                    SmokePing Inspection: <span className="text-primary">{selectedAgent.name}</span>
                  </CardTitle>
                  {getGradeBadge(selectedAgent.quality?.quality_grade || 'A+')}
                </div>
                <CardDescription className="text-xs mt-0.5">
                  Multi-ping dispersion plume (smoke), median latency, and packet loss timeline
                </CardDescription>
              </div>

              {/* Time range selector & Traceroute Button */}
              <div className="flex flex-wrap items-center gap-2 self-start md:self-auto">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleOpenLatestTraceroute}
                  className="gap-1.5 text-xs h-7.5 border-primary/40 text-primary hover:bg-primary/10 hover:text-primary cursor-pointer"
                  title="View NextTrace route hops for this probe"
                >
                  <Route className="size-3.5" />
                  <span>NextTrace Route</span>
                </Button>

                <div className="flex rounded-md border border-border bg-muted/40 p-0.5">
                  {['1h', '6h', '24h', '7d'].map((r) => (
                    <button
                      key={r}
                      onClick={() => setChartRange(r)}
                      className={`rounded px-2.5 py-1 text-xs font-medium transition cursor-pointer ${
                        chartRange === r ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'
                      }`}
                    >
                      {r}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {/* Target selector tabs */}
            {targets.length > 0 && (
              <div className="flex flex-wrap items-center gap-1.5 pt-1 border-t border-border/40">
                <span className="text-xs font-medium text-muted-foreground mr-1 flex items-center gap-1">
                  <Target className="size-3.5 text-primary" /> Reference Target:
                </span>
                {targets.map((tg) => (
                  <button
                    key={tg.id}
                    onClick={() => setSelectedTargetId(tg.id)}
                    className={`rounded-md px-2.5 py-1 text-xs font-medium transition cursor-pointer border ${
                      (selectedTargetId === tg.id || (!selectedTargetId && tg.id === targets[0].id))
                        ? 'bg-primary/20 text-primary border-primary/50'
                        : 'bg-muted/30 text-muted-foreground border-transparent hover:text-foreground hover:bg-muted/60'
                    }`}
                  >
                    {tg.name}
                    <span className="ml-1.5 text-[10px] font-mono opacity-70">({tg.host})</span>
                  </button>
                ))}
              </div>
            )}
          </CardHeader>

          <CardContent className="pt-2">
            <SmokepingChart
              data={chartData}
              probeName={selectedAgent.name}
              targetName={selectedTarget?.name}
              loading={chartLoading}
              onSelectTime={handleChartTimeClick}
            />
          </CardContent>
        </Card>
      )}

      {/* Cross-Probe Matrix: Probes vs Reference Targets */}
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Probes Benchmark Connectivity Matrix</CardTitle>
          <CardDescription className="text-xs">
            Direct comparison showing how each distributed probe performs when pinging reference network targets
          </CardDescription>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="py-8 text-center text-sm text-muted-foreground">Loading matrix...</div>
          ) : agents.length === 0 || targets.length === 0 ? (
            <div className="py-8 text-center text-sm text-muted-foreground">
              Add benchmark targets and deploy agents to see cross-connectivity.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="min-w-[160px]">Probing Agent Node</TableHead>
                    {targets.map((t) => (
                      <TableHead key={t.id} className="text-center min-w-[130px]">
                        <div>{t.name}</div>
                        <div className="text-[10px] text-muted-foreground font-mono">{t.host}</div>
                      </TableHead>
                    ))}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {agents.map((agent) => (
                    <TableRow key={agent.id}>
                      <TableCell className="font-medium">
                        <div className="flex items-center gap-2">
                          <span
                            className={`size-2 rounded-full ${
                              agent.is_online ? 'bg-emerald-500' : 'bg-zinc-600'
                            }`}
                          />
                          <div className="font-semibold text-xs">{agent.name}</div>
                        </div>
                        <div className="text-[11px] text-muted-foreground font-mono pl-4">
                          {agent.public_ip || 'unknown IP'}
                        </div>
                      </TableCell>

                      {targets.map((target) => {
                        const cell = getMatrixCell(agent.id, target.id)
                        if (!cell) {
                          return (
                            <TableCell key={target.id} className="text-center text-xs text-muted-foreground">
                              —
                            </TableCell>
                          )
                        }

                        let color = 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20'
                        if (cell.loss_pct > 10 || cell.avg_rtt_ms > 300) {
                          color = 'text-destructive bg-destructive/10 border-destructive/20'
                        } else if (cell.loss_pct > 0 || cell.jitter_ms > 30 || cell.avg_rtt_ms > 150) {
                          color = 'text-amber-400 bg-amber-500/10 border-amber-500/20'
                        }

                        return (
                          <TableCell key={target.id} className="text-center">
                            <div
                              onClick={() => {
                                setSelectedAgentId(agent.id)
                                setSelectedTargetId(target.id)
                                inspectCardRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
                              }}
                              className={`inline-flex flex-col items-center justify-center rounded-md border px-2.5 py-1 text-xs cursor-pointer transition hover:scale-105 ${color}`}
                            >
                              <div className="font-bold">{cell.avg_rtt_ms} ms</div>
                              <div className="text-[10px] opacity-80">
                                Jitter {cell.jitter_ms}ms | Loss {cell.loss_pct}%
                              </div>
                            </div>
                          </TableCell>
                        )
                      })}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* NextTrace Route Telemetry Modal */}
      {tracerouteModalOpen && (
        <TracerouteDetail
          record={selectedTraceroute}
          loading={tracerouteLoading}
          onClose={() => setTracerouteModalOpen(false)}
          agentName={selectedAgent?.name}
          targetName={selectedTarget?.name}
          agentId={selectedAgentId}
          targetId={selectedTargetId}
          onSelectRecord={handleSelectTracerouteRecord}
        />
      )}
    </div>
  )
}
