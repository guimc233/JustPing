import React from 'react'
import {
  ResponsiveContainer,
  ComposedChart,
  Area,
  Line,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Cell,
} from 'recharts'
import { Flame, AlertTriangle } from 'lucide-react'

export interface MetricDataPoint {
  timestamp: string | Date
  avg_rtt_ms: number
  min_rtt_ms: number
  max_rtt_ms: number
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

// SmokePing 10-step / standard loss palette
const getSmokepingLossColor = (loss: number): string => {
  if (loss <= 0) return '#22c55e' // 0% Green
  if (loss <= 5) return '#06b6d4' // 1-5% Cyan
  if (loss <= 15) return '#3b82f6' // 5-15% Blue
  if (loss <= 25) return '#8b5cf6' // 15-25% Violet
  if (loss <= 50) return '#d946ef' // 25-50% Magenta
  if (loss < 100) return '#f97316' // 50-99% Orange
  return '#ef4444' // 100% Outage Red
}

const SMOKEPING_LOSS_LEGEND = [
  { label: '0%', color: '#22c55e' },
  { label: '1-5%', color: '#06b6d4' },
  { label: '5-15%', color: '#3b82f6' },
  { label: '15-25%', color: '#8b5cf6' },
  { label: '25-50%', color: '#d946ef' },
  { label: '>50%', color: '#f97316' },
  { label: '100% Loss', color: '#ef4444' },
]

interface TooltipPayloadItem {
  payload: {
    time: string
    timestamp: string
    avg: number | null
    min: number | null
    max: number | null
    jitter: number
    loss: number
    isOutage: boolean
    lossColor: string
    errorMsg?: string
  }
}

const SmokepingTooltip = ({
  active,
  payload,
  probeName,
  targetName,
}: {
  active?: boolean
  payload?: TooltipPayloadItem[]
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
        <span className="text-[11px] font-mono text-zinc-400">{d.time}</span>
      </div>

      {d.isOutage ? (
        <div className="flex items-center gap-2 py-1 text-red-400 font-medium">
          <AlertTriangle className="size-4 shrink-0" />
          <span>Complete Outage (100% Packet Loss / Timeout)</span>
        </div>
      ) : (
        <div className="space-y-1.5 font-mono">
          <div className="flex items-center justify-between gap-4">
            <span className="text-zinc-400">Median / Avg RTT:</span>
            <span className="font-bold text-sky-400">{d.avg?.toFixed(2)} ms</span>
          </div>
          <div className="flex items-center justify-between gap-4">
            <span className="text-zinc-400">Dispersion (Smoke):</span>
            <span className="text-zinc-200">
              {d.min?.toFixed(1)} ~ {d.max?.toFixed(1)} ms
              <span className="text-zinc-400 text-[10px] ml-1">
                (Δ{((d.max ?? 0) - (d.min ?? 0)).toFixed(1)}ms)
              </span>
            </span>
          </div>
          <div className="flex items-center justify-between gap-4">
            <span className="text-zinc-400">Jitter (RFC 3550):</span>
            <span className="text-zinc-300">{d.jitter?.toFixed(2)} ms</span>
          </div>
          <div className="flex items-center justify-between gap-4 pt-1 border-t border-zinc-800/80">
            <span className="text-zinc-400">Packet Loss:</span>
            <div className="flex items-center gap-1.5">
              <span
                className="inline-block size-2 rounded-full"
                style={{ backgroundColor: d.lossColor }}
              />
              <span className="font-bold" style={{ color: d.lossColor }}>
                {d.loss.toFixed(1)}%
              </span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export const SmokepingChart: React.FC<SmokepingChartProps> = ({
  data,
  probeName,
  targetName,
  loading,
}) => {
  if (loading) {
    return (
      <div className="h-[280px] flex flex-col items-center justify-center gap-2 text-xs text-muted-foreground">
        <Flame className="size-5 text-primary animate-pulse" />
        <span>Synthesizing SmokePing dispersion metrics...</span>
      </div>
    )
  }

  if (!data || data.length === 0) {
    return (
      <div className="h-[280px] flex items-center justify-center text-xs text-muted-foreground">
        No latency data recorded yet for this target in the selected period.
      </div>
    )
  }

  // Find max latency to size the 100% loss outage pillar appropriately
  let maxLatency = 50
  for (const pt of data) {
    if (pt.max_rtt_ms > maxLatency) maxLatency = pt.max_rtt_ms
    if (pt.avg_rtt_ms > maxLatency) maxLatency = pt.avg_rtt_ms
  }
  const outageHeight = Math.ceil(maxLatency * 1.15)

  // Transform into SmokePing multi-layer dispersion representation
  const chartPoints = data.map((d) => {
    const isOutage = d.loss_pct >= 100
    const timeStr = new Date(d.timestamp).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
    })

    const min = d.min_rtt_ms
    const max = d.max_rtt_ms
    const avg = d.avg_rtt_ms
    const spread = d.jitter_ms || 0

    // Core smoke bounds (interquartile / std dev equivalent)
    const coreLower = Math.max(min, avg - spread)
    const coreUpper = Math.min(max, avg + spread)

    const lossColor = getSmokepingLossColor(d.loss_pct)

    return {
      time: timeStr,
      timestamp: String(d.timestamp),
      avg: isOutage ? null : avg,
      min: isOutage ? null : min,
      max: isOutage ? null : max,
      jitter: d.jitter_ms,
      loss: d.loss_pct,
      isOutage,
      lossColor,
      // Range areas: [lowerBound, upperBound]
      outerSmoke: isOutage ? null : [min, max],
      innerSmoke: isOutage ? null : [coreLower, coreUpper],
      // Full height pillar when 100% loss occurs
      outage: isOutage ? outageHeight : null,
      // Bottom loss indicator tick height
      lossTick: d.loss_pct > 0 ? 1 : 0.25,
      errorMsg: d.error_msg,
    }
  })

  return (
    <div className="flex flex-col gap-2 w-full">
      {/* Chart Canvas */}
      <div className="h-[270px] w-full">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart
            data={chartPoints}
            margin={{ top: 10, right: 12, left: -20, bottom: 0 }}
          >
            <CartesianGrid strokeDasharray="3 3" stroke="#27272a" vertical={false} />
            <XAxis
              dataKey="time"
              stroke="#71717a"
              fontSize={11}
              tickLine={false}
              minTickGap={25}
            />
            <YAxis
              yAxisId="rtt"
              stroke="#71717a"
              fontSize={11}
              unit="ms"
              domain={[0, 'auto']}
              tickLine={false}
            />
            {/* Secondary hidden YAxis for the bottom loss strip */}
            <YAxis yAxisId="loss" domain={[0, 5]} hide />

            <Tooltip
              content={
                <SmokepingTooltip probeName={probeName} targetName={targetName} />
              }
            />

            {/* Outage Bars (100% packet loss red vertical pillars) */}
            <Bar
              yAxisId="rtt"
              dataKey="outage"
              fill="#ef4444"
              fillOpacity={0.45}
              isAnimationActive={false}
              name="100% Loss Outage"
            />

            {/* Outer Smoke: Dispersion whisker between Min and Max RTT */}
            <Area
              yAxisId="rtt"
              type="monotone"
              dataKey="outerSmoke"
              stroke="none"
              fill="#64748b"
              fillOpacity={0.22}
              isAnimationActive={false}
              name="Smoke (Min-Max)"
              connectNulls={false}
            />

            {/* Inner Smoke: Dense core around median / avg */}
            <Area
              yAxisId="rtt"
              type="monotone"
              dataKey="innerSmoke"
              stroke="none"
              fill="#94a3b8"
              fillOpacity={0.45}
              isAnimationActive={false}
              name="Smoke (Core)"
              connectNulls={false}
            />

            {/* Center Median / Avg Line */}
            <Line
              yAxisId="rtt"
              type="monotone"
              dataKey="avg"
              stroke="#38bdf8"
              strokeWidth={1.75}
              dot={false}
              isAnimationActive={false}
              name="Median / Avg Latency"
              connectNulls={false}
            />

            {/* SmokePing bottom color strip indicating packet loss */}
            <Bar
              yAxisId="loss"
              dataKey="lossTick"
              isAnimationActive={false}
              barSize={4}
              name="Packet Loss"
            >
              {chartPoints.map((entry, idx) => (
                <Cell key={`loss-${idx}`} fill={entry.lossColor} />
              ))}
            </Bar>
          </ComposedChart>
        </ResponsiveContainer>
      </div>

      {/* SmokePing Loss Palette Color Key Legend */}
      <div className="flex flex-wrap items-center justify-between border-t border-border/40 pt-2 text-[11px] text-muted-foreground gap-2">
        <div className="flex items-center gap-1.5">
          <span className="font-medium text-foreground">SmokePing Packet Loss:</span>
          <div className="flex items-center gap-1 sm:gap-2">
            {SMOKEPING_LOSS_LEGEND.map((item) => (
              <div key={item.label} className="flex items-center gap-1">
                <span
                  className="size-2 rounded-sm shrink-0"
                  style={{ backgroundColor: item.color }}
                />
                <span className="text-[10px] text-zinc-300">{item.label}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-3 text-[10px]">
          <span className="flex items-center gap-1">
            <span className="size-2 rounded-sm bg-sky-400" /> Median Latency
          </span>
          <span className="flex items-center gap-1">
            <span className="size-2 rounded-sm bg-slate-400/50" /> Smoke Plume (Dispersion)
          </span>
        </div>
      </div>
    </div>
  )
}
