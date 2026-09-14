import React, { useState, useEffect } from 'react'
import { Card, CardHeader, CardTitle, CardContent } from './ui/card'
import { Button } from './ui/button'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from './ui/table'
import { Activity, Server, Radio, Zap, Shield, RefreshCw } from 'lucide-react'
import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip, CartesianGrid } from 'recharts'

interface PublicDashboardProps {
  isAdmin: boolean
}

export const PublicDashboard: React.FC<PublicDashboardProps> = ({ isAdmin }) => {
  const [summary, setSummary] = useState<any>(null)
  const [agents, setAgents] = useState<any[]>([])
  const [targets, setTargets] = useState<any[]>([])
  const [matrix, setMatrix] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  // Chart state
  const [selectedAgent, setSelectedAgent] = useState<string>('')
  const [selectedTarget, setSelectedTarget] = useState<string>('')
  const [chartRange, setChartRange] = useState<string>('1h')
  const [chartData, setChartData] = useState<any[]>([])
  const [chartLoading, setChartLoading] = useState(false)

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

      if (agData.length > 0 && !selectedAgent) {
        setSelectedAgent(agData[0].id)
      }
      if (tgData.length > 0 && !selectedTarget) {
        setSelectedTarget(tgData[0].id)
      }
    } catch (err) {
      console.error('Failed to load public metrics:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchMetrics = async () => {
    if (!selectedTarget) return
    setChartLoading(true)
    try {
      let url = `/api/public/metrics?range=${chartRange}&target_id=${selectedTarget}`
      if (selectedAgent) {
        url += `&agent_id=${selectedAgent}`
      }
      const res = await fetch(url)
      const data = await res.json()
      const formatted = data.map((d: any) => ({
        time: new Date(d.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        avg: d.avg_rtt_ms,
        min: d.min_rtt_ms,
        max: d.max_rtt_ms,
        jitter: d.jitter_ms,
        loss: d.loss_pct,
      }))
      setChartData(formatted)
    } catch (err) {
      console.error('Failed to load chart metrics:', err)
    } finally {
      setChartLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 15000) // 15s refresh
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    fetchMetrics()
  }, [selectedAgent, selectedTarget, chartRange])

  const getMatrixCell = (agentId: string, targetId: string) => {
    return matrix.find((m) => m.agent_id === agentId && m.target_id === targetId)
  }

  return (
    <div className="flex flex-col gap-6">
      {/* Privacy Notice Banner */}
      {!isAdmin && (
        <div className="flex items-center justify-between rounded-lg border border-border/80 bg-muted/30 px-4 py-2.5 text-xs text-muted-foreground">
          <div className="flex items-center gap-2">
            <Shield className="size-4 text-primary" />
            <span>
              Public view active: IP addresses and private infrastructure topologies are automatically masked.
            </span>
          </div>
          <Button variant="ghost" size="sm" onClick={fetchData} className="size-7 p-0">
            <RefreshCw className="size-3.5" />
          </Button>
        </div>
      )}

      {/* Summary Cards */}
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
            <div className="text-xs font-medium text-muted-foreground">Monitored Targets</div>
            <div className="text-2xl font-bold mt-1">{summary?.total_targets || 0}</div>
          </div>
          <div className="size-10 rounded-lg bg-blue-500/10 text-blue-400 flex items-center justify-center">
            <Radio className="size-5" />
          </div>
        </Card>

        <Card className="p-4 flex items-center justify-between">
          <div>
            <div className="text-xs font-medium text-muted-foreground">Global Avg Latency</div>
            <div className="text-2xl font-bold mt-1">
              {summary?.avg_rtt_ms ? Number(summary.avg_rtt_ms).toFixed(1) : 0}{' '}
              <span className="text-xs font-normal text-muted-foreground">ms</span>
            </div>
          </div>
          <div className="size-10 rounded-lg bg-purple-500/10 text-purple-400 flex items-center justify-center">
            <Activity className="size-5" />
          </div>
        </Card>

        <Card className="p-4 flex items-center justify-between">
          <div>
            <div className="text-xs font-medium text-muted-foreground">Avg Packet Loss</div>
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

      {/* Latency & Loss Matrix Heatmap */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-2">
          <div>
            <CardTitle>Probe Latency & Loss Matrix</CardTitle>
            <div className="text-xs text-muted-foreground mt-0.5">
              Live status cross-tested between distributed probes and ICMP targets
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="py-12 text-center text-sm text-muted-foreground">Loading matrix data...</div>
          ) : agents.length === 0 || targets.length === 0 ? (
            <div className="py-12 text-center text-sm text-muted-foreground">
              No agents or targets active. Install an agent or add ping targets in Admin Panel.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="min-w-[160px]">Probe / Agent</TableHead>
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
                          {agent.public_ip || 'unknown'}
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
                                setSelectedAgent(agent.id)
                                setSelectedTarget(target.id)
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

      {/* Latency History Chart */}
      <Card>
        <CardHeader className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-2">
          <div>
            <CardTitle>Network Quality Trends</CardTitle>
            <div className="text-xs text-muted-foreground mt-0.5">
              Historical latency, jitter, and packet loss metrics
            </div>
          </div>

          {/* Filter Bar */}
          <div className="flex flex-wrap items-center gap-2">
            <select
              value={selectedAgent}
              onChange={(e) => setSelectedAgent(e.target.value)}
              className="h-8 rounded-md border border-input bg-card px-2 text-xs"
            >
              {agents.map((a) => (
                <option key={a.id} value={a.id}>
                  Probe: {a.name}
                </option>
              ))}
            </select>

            <select
              value={selectedTarget}
              onChange={(e) => setSelectedTarget(e.target.value)}
              className="h-8 rounded-md border border-input bg-card px-2 text-xs"
            >
              {targets.map((t) => (
                <option key={t.id} value={t.id}>
                  Target: {t.name}
                </option>
              ))}
            </select>

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
        </CardHeader>

        <CardContent className="pt-4">
          {chartLoading ? (
            <div className="h-[280px] flex items-center justify-center text-xs text-muted-foreground">
              Loading metrics...
            </div>
          ) : chartData.length === 0 ? (
            <div className="h-[280px] flex items-center justify-center text-xs text-muted-foreground">
              No historical data recorded yet for this combination.
            </div>
          ) : (
            <div className="h-[300px] w-full">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                  <defs>
                    <linearGradient id="colorAvg" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.4} />
                      <stop offset="95%" stopColor="#3b82f6" stopOpacity={0.0} />
                    </linearGradient>
                    <linearGradient id="colorJitter" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="#a855f7" stopOpacity={0.3} />
                      <stop offset="95%" stopColor="#a855f7" stopOpacity={0.0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="#27272a" />
                  <XAxis dataKey="time" stroke="#71717a" fontSize={11} />
                  <YAxis stroke="#71717a" fontSize={11} unit="ms" />
                  <Tooltip
                    contentStyle={{ backgroundColor: '#18181b', borderColor: '#27272a', borderRadius: '8px' }}
                    labelStyle={{ color: '#e4e4e7', fontWeight: 'bold', fontSize: '12px' }}
                  />
                  <Area
                    type="monotone"
                    dataKey="avg"
                    name="Avg Latency"
                    stroke="#3b82f6"
                    strokeWidth={2}
                    fillOpacity={1}
                    fill="url(#colorAvg)"
                  />
                  <Area
                    type="monotone"
                    dataKey="jitter"
                    name="Jitter"
                    stroke="#a855f7"
                    strokeWidth={1.5}
                    fillOpacity={1}
                    fill="url(#colorJitter)"
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
