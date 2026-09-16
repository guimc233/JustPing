import React, { useState, useEffect } from 'react'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from './ui/table'
import {
  Route,
  Clock,
  MapPin,
  CheckCircle2,
  XCircle,
  X,
  History,
  Shield,
  Loader2,
} from 'lucide-react'

export interface EnrichedHop {
  ttl: number
  ip: string
  hostname?: string
  rtts_ms: number[]
  avg_rtt_ms: number
  loss_pct: number
  as_number?: string
  as_org?: string
  isp?: string
  country?: string
  country_code?: string
  city?: string
}

export interface TracerouteRecord {
  id: string
  agent_id: string
  target_id: string
  target_host: string
  resolved_ip: string
  timestamp: string
  duration_ms: number
  reached: boolean
  hop_count: number
  route_path?: string
  hops: EnrichedHop[]
}

interface TracerouteDetailProps {
  record: TracerouteRecord | null
  loading?: boolean
  onClose: () => void
  agentName?: string
  targetName?: string
  agentId?: string
  targetId?: string
  onSelectRecord?: (recordId: string) => void
}

export const TracerouteDetail: React.FC<TracerouteDetailProps> = ({
  record,
  loading = false,
  onClose,
  agentName,
  targetName,
  agentId,
  targetId,
  onSelectRecord,
}) => {
  const [historyList, setHistoryList] = useState<any[]>([])

  // Fetch recent history list for dropdown / quick jump
  useEffect(() => {
    if (!agentId) return
    const fetchHistory = async () => {
      try {
        let url = `/api/public/traceroutes?agent_id=${agentId}`
        if (targetId) url += `&target_id=${targetId}`
        const res = await fetch(url)
        if (res.ok) {
          const list = await res.json()
          setHistoryList(list)
        }
      } catch (err) {
        console.error('Failed to load traceroute history:', err)
      }
    }
    fetchHistory()
  }, [agentId, targetId])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 overflow-y-auto">
      <div className="relative w-full max-w-5xl rounded-xl border border-border bg-background shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border bg-muted/30 px-6 py-4">
          <div className="flex items-center gap-3">
            <div className="size-9 rounded-lg bg-primary/10 text-primary flex items-center justify-center">
              <Route className="size-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-base font-bold text-foreground">
                  NextTrace Route Telemetry: {agentName || 'Probe'} → {targetName || record?.target_host || 'Target'}
                </h2>
                {record?.route_path && (
                  <Badge variant="secondary" className="font-bold text-[11px] bg-amber-500/15 text-amber-300 border-amber-500/30">
                    {record.route_path}
                  </Badge>
                )}
                {record?.reached ? (
                  <Badge variant="success" className="gap-1 text-[10px]">
                    <CheckCircle2 className="size-3" /> Reached (到达目标)
                  </Badge>
                ) : record ? (
                  <Badge variant="destructive" className="gap-1 text-[10px]">
                    <XCircle className="size-3" /> Max Hops (30) / Incomplete
                  </Badge>
                ) : null}
              </div>
              <p className="text-xs text-muted-foreground mt-0.5">
                Detailed intermediate AS routing, geographic hops, and round-trip times (max 30 hops)
              </p>
            </div>
          </div>

          <Button variant="ghost" size="icon" onClick={onClose} className="size-8">
            <X className="size-4" />
          </Button>
        </div>

        {/* Top Metadata Bar & History Selector */}
        <div className="border-b border-border/80 bg-muted/15 px-6 py-3 flex flex-wrap items-center justify-between gap-4 text-xs">
          <div className="flex flex-wrap items-center gap-4 text-muted-foreground">
            <div>
              <span className="text-zinc-500">Target Host:</span>{' '}
              <span className="font-mono font-medium text-foreground">{record?.target_host || '—'}</span>{' '}
              {record?.resolved_ip && (
                <span className="font-mono text-zinc-400">({record.resolved_ip})</span>
              )}
            </div>
            <div className="flex items-center gap-1.5">
              <Clock className="size-3.5 text-zinc-400" />
              <span>
                {record?.timestamp ? new Date(record.timestamp).toLocaleString() : '—'}
              </span>
            </div>
            <div>
              <span className="text-zinc-500">Duration:</span>{' '}
              <span className="font-mono text-foreground">{record?.duration_ms || 0} ms</span>
            </div>
            <div>
              <span className="text-zinc-500">Hops:</span>{' '}
              <span className="font-mono text-foreground">{record?.hops?.length || 0} / 30</span>
            </div>
          </div>

          {/* History Selection Dropdown */}
          {historyList.length > 0 && (
            <div className="flex items-center gap-2">
              <span className="text-zinc-500 text-[11px] flex items-center gap-1">
                <History className="size-3" /> Route History:
              </span>
              <select
                value={record?.id || ''}
                onChange={(e) => onSelectRecord?.(e.target.value)}
                className="rounded-md border border-border bg-background px-2.5 py-1 text-xs font-mono text-foreground focus:outline-none focus:ring-1 focus:ring-primary cursor-pointer"
              >
                {historyList.map((h) => (
                  <option key={h.id} value={h.id}>
                    {new Date(h.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    {h.route_path ? ` [${h.route_path}]` : ''} ({h.hop_count} hops, {h.reached ? 'Reached' : 'Max 30'})
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>

        {/* Content Body */}
        <div className="flex-1 overflow-y-auto p-6">
          {loading ? (
            <div className="flex flex-col items-center justify-center py-20 gap-3 text-muted-foreground">
              <Loader2 className="size-6 animate-spin text-primary" />
              <span className="text-xs">Querying NextTrace hop telemetry...</span>
            </div>
          ) : !record || !record.hops || record.hops.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 gap-2 text-center text-muted-foreground">
              <Route className="size-8 text-zinc-600" />
              <span className="text-sm font-medium">No route trace recorded for this period.</span>
              <span className="text-xs max-w-sm">
                Route traces are executed sequentially every 15 minutes. Click another time range on the chart to inspect.
              </span>
            </div>
          ) : (
            <div className="rounded-lg border border-border/80 overflow-hidden bg-card/60">
              <Table>
                <TableHeader className="bg-muted/40 text-[11px]">
                  <TableRow>
                    <TableHead className="w-12 text-center">#</TableHead>
                    <TableHead className="min-w-[160px]">IP Address / Hostname</TableHead>
                    <TableHead className="min-w-[120px]">RTT Attempts</TableHead>
                    <TableHead className="w-20 text-center">Avg RTT</TableHead>
                    <TableHead className="w-16 text-center">Loss</TableHead>
                    <TableHead className="min-w-[140px]">ASN & Org</TableHead>
                    <TableHead className="min-w-[160px]">Location & ISP</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody className="text-xs font-mono">
                  {record.hops.map((hop) => {
                    const isTimeout = !hop.ip || hop.ip === '*'
                    const isLAN = hop.as_org === 'Private / LAN' || hop.country === 'Local'

                    return (
                      <TableRow key={hop.ttl} className={isTimeout ? 'opacity-60 bg-muted/10' : ''}>
                        {/* Hop number */}
                        <TableCell className="text-center font-bold text-zinc-400">
                          {hop.ttl}
                        </TableCell>

                        {/* IP Address & Hostname */}
                        <TableCell>
                          {isTimeout ? (
                            <span className="text-zinc-500 font-bold">* * * (Timeout)</span>
                          ) : (
                            <div className="space-y-0.5">
                              <div className="font-semibold text-foreground flex items-center gap-1.5">
                                <span>{hop.ip}</span>
                                {isLAN && (
                                  <span className="rounded bg-zinc-800 px-1.5 py-0.2 text-[9px] text-zinc-400">
                                    LAN
                                  </span>
                                )}
                              </div>
                              {hop.hostname && (
                                <div className="text-[10px] text-zinc-400 font-normal truncate max-w-xs">
                                  {hop.hostname}
                                </div>
                              )}
                            </div>
                          )}
                        </TableCell>

                        {/* Probe attempts RTTs */}
                        <TableCell className="text-[11px]">
                          {isTimeout || !hop.rtts_ms || hop.rtts_ms.length === 0 ? (
                            <span className="text-zinc-500">*</span>
                          ) : (
                            <div className="flex items-center gap-1 flex-wrap">
                              {hop.rtts_ms.map((r, i) => (
                                <span key={i} className="text-sky-400/90 font-medium">
                                  {r.toFixed(1)}ms{i < hop.rtts_ms.length - 1 ? ',' : ''}
                                </span>
                              ))}
                            </div>
                          )}
                        </TableCell>

                        {/* Avg RTT */}
                        <TableCell className="text-center">
                          {isTimeout || hop.avg_rtt_ms <= 0 ? (
                            <span className="text-zinc-500">—</span>
                          ) : (
                            <span className="font-bold text-sky-400">{hop.avg_rtt_ms.toFixed(1)} ms</span>
                          )}
                        </TableCell>

                        {/* Loss % */}
                        <TableCell className="text-center">
                          <span
                            className={`font-semibold ${
                              hop.loss_pct === 0
                                ? 'text-emerald-400'
                                : hop.loss_pct < 100
                                ? 'text-amber-400'
                                : 'text-red-400'
                            }`}
                          >
                            {hop.loss_pct}%
                          </span>
                        </TableCell>

                        {/* ASN & Org */}
                        <TableCell>
                          {hop.as_number && hop.as_number !== '*' ? (
                            <div className="flex flex-col gap-0.5 font-sans">
                              <span className="inline-flex items-center gap-1">
                                <Badge variant="secondary" className="px-1.5 py-0 text-[10px] font-mono text-sky-300">
                                  {hop.as_number}
                                </Badge>
                              </span>
                              {hop.as_org && (
                                <span className="text-[11px] text-muted-foreground truncate max-w-[180px]">
                                  {hop.as_org}
                                </span>
                              )}
                            </div>
                          ) : isLAN ? (
                            <span className="text-zinc-500 text-[11px] font-sans">Private Network</span>
                          ) : (
                            <span className="text-zinc-600">—</span>
                          )}
                        </TableCell>

                        {/* Location & ISP */}
                        <TableCell>
                          {isTimeout ? (
                            <span className="text-zinc-600">—</span>
                          ) : (
                            <div className="space-y-0.5 font-sans text-[11px]">
                              {(hop.country || hop.city) && (
                                <div className="text-foreground flex items-center gap-1 font-medium">
                                  <MapPin className="size-3 text-red-400 shrink-0" />
                                  <span>
                                    {[hop.country, hop.city].filter(Boolean).join(', ')}
                                  </span>
                                </div>
                              )}
                              {hop.isp && (
                                <div className="text-muted-foreground text-[10px]">
                                  {hop.isp}
                                </div>
                              )}
                            </div>
                          )}
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="border-t border-border bg-muted/20 px-6 py-3 flex items-center justify-between text-xs text-muted-foreground">
          <div className="flex items-center gap-2">
            <Shield className="size-3.5 text-primary" />
            <span>NextTrace telemetry enriched with ASN and Geo routing data.</span>
          </div>
          <Button size="sm" variant="secondary" onClick={onClose}>
            Close
          </Button>
        </div>
      </div>
    </div>
  )
}
