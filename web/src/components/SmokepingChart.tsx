import React, { useMemo } from 'react'
import {
  ResponsiveContainer,
  ComposedChart,
  Area,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  ReferenceArea,
} from 'recharts'
import { Flame, AlertTriangle, AlertCircle, CheckCircle2 } from 'lucide-react'

export interface MetricDataPoint {
  timestamp: string | Date
  avg_rtt_ms: number
  min_rtt_ms?: number
  max_rtt_ms?: number
  jitter_ms: number
  loss_pct: number
  packets_sent?: number
  packets_recv?: number
  error_msg?: string
}

interface SmokepingChartProps {
  data: MetricDataPoint[]
  probeName?: string
  targetName?: string
  loading?: boolean
}

interface MinuteSlot {
  timeKey: string // unique timestamp string as key
  timestampMs: number
  timeLabel: string
  fullDateLabel: string
  avg: number | null
  jitter: number
  hasLoss: boolean
  status: 'ok' | 'partial_loss' | 'outage' | 'no_data'
  errorMsg?: string
}

interface LossInterval {
  startKey: string
  endKey: string
  lengthMinutes: number
  isPersistent: boolean
}

const SmokepingTooltip = ({
  active,
  payload,
  probeName,
  targetName,
}: {
  active?: boolean
  payload?: Array<{ payload: MinuteSlot }>
  probeName?: string
  targetName?: string
}) => {
  if (!active || !payload || !payload.length) return null
  const d = payload[0].payload

  return (
    <div className="rounded-lg border border-border/80 bg-zinc-950/95 p-3 text-xs shadow-xl backdrop-blur-sm">
      <div className="flex items-center justify-between gap-4 border-b border-zinc-800 pb-2 mb-2">
        <div>
          <span className="font-semibold text-zinc-100">{probeName || 'Probe'}</span>
          {targetName && (
            <span className="text-zinc-400"> → <span className="text-sky-400 font-medium">{targetName}</span></span>
          )}
        </div>
        <span className="text-[11px] font-mono text-zinc-400">{d.timeLabel}</span>
      </div>

      <div className="space-y-1.5 font-mono">
        <div className="text-[10px] text-zinc-500 mb-1">{d.fullDateLabel}</div>

        {d.status === 'no_data' ? (
          <div className="flex items-center gap-2 py-1 text-amber-400 font-medium">
            <AlertTriangle className="size-4 shrink-0 text-red-400" />
            <span className="text-red-400">No ping returned in this minute (Disconnected)</span>
          </div>
        ) : d.status === 'outage' ? (
          <div className="flex items-center gap-2 py-1 text-red-400 font-medium">
            <AlertTriangle className="size-4 shrink-0 text-red-400" />
            <span>Packet timeout (100% loss / unreachable)</span>
          </div>
        ) : (
          <>
            <div className="flex items-center justify-between gap-4">
              <span className="text-zinc-400">Latency (RTT):</span>
              <span className="font-bold text-sky-400">
                {d.avg !== null ? `${d.avg.toFixed(2)} ms` : '—'}
              </span>
            </div>
            {d.jitter > 0 && (
              <div className="flex items-center justify-between gap-4">
                <span className="text-zinc-400">Jitter (RFC 3550):</span>
                <span className="text-zinc-300">{d.jitter.toFixed(2)} ms</span>
              </div>
            )}
            <div className="flex items-center justify-between gap-4 pt-1 border-t border-zinc-800/80">
              <span className="text-zinc-400">Network State:</span>
              {d.status === 'partial_loss' ? (
                <div className="flex items-center gap-1.5 text-amber-400 font-medium">
                  <AlertCircle className="size-3.5" />
                  <span>Packet loss detected</span>
                </div>
              ) : (
                <div className="flex items-center gap-1.5 text-emerald-400 font-medium">
                  <CheckCircle2 className="size-3.5" />
                  <span>Normal (No loss)</span>
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  )
}

export const SmokepingChart: React.FC<SmokepingChartProps> = ({
  data,
  probeName,
  targetName,
  loading,
}) => {
  const { chartSlots, lossIntervals, spanHours } = useMemo(() => {
    if (!data || data.length === 0) {
      return { chartSlots: [], lossIntervals: [], spanHours: 1 }
    }

    // Sort ascending by timestamp
    const sorted = [...data].sort(
      (a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
    )

    const firstMs = new Date(sorted[0].timestamp).getTime()
    const lastMs = new Date(sorted[sorted.length - 1].timestamp).getTime()

    const endMinuteMs = Math.floor(lastMs / 60000) * 60000
    const startMinuteMs = Math.floor(firstMs / 60000) * 60000

    const totalSpanMinutes = Math.max(1, Math.round((endMinuteMs - startMinuteMs) / 60000))
    const calculatedHours = totalSpanMinutes / 60

    // Step size: 1 minute for spans up to 24h, 5 minutes for spans > 24h (e.g. 7d)
    const stepMs = totalSpanMinutes > 1440 ? 5 * 60000 : 60000

    // Index points into step buckets
    const bucketMap = new Map<number, MetricDataPoint[]>()
    for (const pt of sorted) {
      const ptMs = new Date(pt.timestamp).getTime()
      const bucketKey = Math.floor(ptMs / stepMs) * stepMs
      const existing = bucketMap.get(bucketKey)
      if (existing) {
        existing.push(pt)
      } else {
        bucketMap.set(bucketKey, [pt])
      }
    }

    const slots: MinuteSlot[] = []

    for (let m = startMinuteMs; m <= endMinuteMs; m += stepMs) {
      const pings = bucketMap.get(m)
      const d = new Date(m)
      const timeLabel = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      const fullDateLabel = d.toLocaleString([], {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })

      if (!pings || pings.length === 0) {
        // No ping returned in this minute slot: disconnect line & flag as missing/loss
        slots.push({
          timeKey: String(m),
          timestampMs: m,
          timeLabel,
          fullDateLabel,
          avg: null,
          jitter: 0,
          hasLoss: true,
          status: 'no_data',
        })
        continue
      }

      // Check for successful packets
      const valid = pings.filter((p) => p.loss_pct < 100 && p.avg_rtt_ms > 0)
      if (valid.length === 0) {
        // 100% loss or timeout in this minute: disconnect line & flag as loss
        slots.push({
          timeKey: String(m),
          timestampMs: m,
          timeLabel,
          fullDateLabel,
          avg: null,
          jitter: 0,
          hasLoss: true,
          status: 'outage',
          errorMsg: pings[0]?.error_msg,
        })
        continue
      }

      // Has at least one received packet
      const avgRTT = Math.round((valid.reduce((s, p) => s + p.avg_rtt_ms, 0) / valid.length) * 100) / 100
      const maxJitter = Math.max(...valid.map((p) => p.jitter_ms || 0))
      const hasAnyLoss = pings.some(
        (p) => p.loss_pct > 0 || (p.packets_recv !== undefined && p.packets_sent !== undefined && p.packets_recv < p.packets_sent)
      )

      slots.push({
        timeKey: String(m),
        timestampMs: m,
        timeLabel,
        fullDateLabel,
        avg: avgRTT,
        jitter: maxJitter,
        hasLoss: hasAnyLoss,
        status: hasAnyLoss ? 'partial_loss' : 'ok',
      })
    }

    // Find contiguous intervals of packet loss
    const intervals: LossInterval[] = []
    let curStart = -1

    for (let i = 0; i < slots.length; i++) {
      if (slots[i].hasLoss) {
        if (curStart === -1) {
          curStart = i
        }
      } else {
        if (curStart !== -1) {
          const count = i - curStart
          intervals.push({
            startKey: slots[curStart].timeKey,
            endKey: slots[i - 1].timeKey,
            lengthMinutes: count,
            isPersistent: count >= 2,
          })
          curStart = -1
        }
      }
    }

    if (curStart !== -1) {
      const count = slots.length - curStart
      intervals.push({
        startKey: slots[curStart].timeKey,
        endKey: slots[slots.length - 1].timeKey,
        lengthMinutes: count,
        isPersistent: count >= 2,
      })
    }

    return { chartSlots: slots, lossIntervals: intervals, spanHours: calculatedHours }
  }, [data])

  if (loading) {
    return (
      <div className="h-[280px] flex flex-col items-center justify-center gap-2 text-xs text-muted-foreground">
        <Flame className="size-5 text-primary animate-pulse" />
        <span>Loading latency metrics...</span>
      </div>
    )
  }

  if (!data || data.length === 0 || chartSlots.length === 0) {
    return (
      <div className="h-[280px] flex items-center justify-center text-xs text-muted-foreground">
        No latency data recorded yet for this target in the selected period.
      </div>
    )
  }

  const formatXAxisTick = (timeKey: string) => {
    const d = new Date(Number(timeKey))
    if (spanHours > 24) {
      const mo = (d.getMonth() + 1).toString().padStart(2, '0')
      const day = d.getDate().toString().padStart(2, '0')
      const hr = d.getHours().toString().padStart(2, '0')
      const min = d.getMinutes().toString().padStart(2, '0')
      return `${mo}-${day} ${hr}:${min}`
    }
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }

  return (
    <div className="flex flex-col gap-2 w-full">
      {/* Chart Canvas */}
      <div className="h-[280px] w-full">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart
            data={chartSlots}
            margin={{ top: 12, right: 20, left: 10, bottom: 6 }}
          >
            <defs>
              <linearGradient id="latencyGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#38bdf8" stopOpacity={0.25} />
                <stop offset="95%" stopColor="#38bdf8" stopOpacity={0.0} />
              </linearGradient>
            </defs>

            <CartesianGrid strokeDasharray="3 3" stroke="#27272a" vertical={false} />

            {/* Native Background Highlight Areas for Packet Loss Periods */}
            {lossIntervals.map((interval, idx) => (
              <ReferenceArea
                key={`loss-bg-${idx}`}
                x1={interval.startKey}
                x2={interval.endKey}
                fill={interval.isPersistent ? '#ef4444' : '#eab308'}
                fillOpacity={interval.isPersistent ? 0.24 : 0.18}
                stroke="none"
                ifOverflow="visible"
              />
            ))}

            <XAxis
              dataKey="timeKey"
              tickFormatter={formatXAxisTick}
              stroke="#71717a"
              fontSize={11}
              tickLine={false}
              minTickGap={35}
            />

            {/* Prominent, readable Vertical Y-Axis with label and units */}
            <YAxis
              stroke="#71717a"
              fontSize={11}
              unit="ms"
              width={55}
              domain={[0, 'auto']}
              tickLine={true}
              axisLine={{ stroke: '#3f3f46' }}
              label={{
                value: 'Latency (ms)',
                angle: -90,
                position: 'insideLeft',
                offset: 12,
                fill: '#a1a1aa',
                fontSize: 11,
                style: { textAnchor: 'middle' },
              }}
            />

            <Tooltip
              content={
                <SmokepingTooltip probeName={probeName} targetName={targetName} />
              }
            />

            {/* Latency Gradient Area (breaks on null) */}
            <Area
              type="monotone"
              dataKey="avg"
              stroke="none"
              fill="url(#latencyGradient)"
              isAnimationActive={false}
              connectNulls={false}
            />

            {/* Latency Curve (breaks cleanly on null whenever a minute has no ping) */}
            <Line
              type="monotone"
              dataKey="avg"
              stroke="#38bdf8"
              strokeWidth={2}
              dot={false}
              isAnimationActive={false}
              name="Latency (RTT)"
              connectNulls={false}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      {/* Clear Legend explaining Status & Background Highlights */}
      <div className="flex flex-wrap items-center justify-between border-t border-border/40 pt-2 text-[11px] text-muted-foreground gap-2">
        <div className="flex items-center gap-4">
          <span className="flex items-center gap-1.5">
            <span className="size-2.5 rounded-sm bg-sky-400" />
            <span className="text-foreground font-medium">Latency (RTT)</span>
          </span>
          <span className="flex items-center gap-1.5">
            <span className="size-2.5 rounded-sm bg-amber-400/80 border border-amber-400" />
            <span>Packet Loss Period (丢包时段)</span>
          </span>
          <span className="flex items-center gap-1.5">
            <span className="size-2.5 rounded-sm bg-red-500/80 border border-red-500" />
            <span>Persistent Loss / Disconnected (持续丢包 / 中断)</span>
          </span>
        </div>

        <div className="text-[10px] text-zinc-500">
          * Missing minute slots will break the line chart
        </div>
      </div>
    </div>
  )
}
