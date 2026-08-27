/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type {
  GroupCacheSummary,
  GroupSummaryAllData,
  PerformanceSeriesPoint,
} from '@/features/performance-metrics/types'

export type AvailabilitySegment = {
  ts: number
  successRate: number
  hasData: boolean
}

export function resolveModelStatusGroups(
  data: GroupSummaryAllData['data'] | undefined
) {
  return Array.from(
    new Set([
      ...(data?.available_groups ?? []),
      ...(data?.groups ?? []).map((summary) => summary.group),
    ])
  ).filter((group) => Boolean(group) && group.toLowerCase() !== 'auto')
}

export function formatStatusGroupRatio(ratio: number | undefined): string {
  if (ratio == null || !Number.isFinite(ratio)) return '—'
  return ratio % 1 === 0
    ? ratio.toFixed(0)
    : ratio.toFixed(4).replace(/\.?0+$/, '')
}

export function hasCompleteHealthMetrics(
  summary: GroupCacheSummary | undefined
) {
  return Number.isFinite(summary?.success_rate)
}

export function summarizeAvailabilitySeries(
  series: PerformanceSeriesPoint[],
  hours: number,
  maxSegments?: number
): AvailabilitySegment[] {
  const safeHours = Math.max(1, hours)
  // Keep the 24-hour view at 48 thirty-minute columns. Longer ranges retain
  // that same density by widening each column (7d = 3h30m, 30d = 15h).
  const safeMaxSegments = Math.max(1, maxSegments ?? 48)
  const segmentSeconds = Math.max(
    1,
    Math.ceil((safeHours * 3600) / safeMaxSegments)
  )
  const buckets = new Map<
    number,
    {
      requests: number
      successes: number
      rateSum: number
      points: number
      hasData: boolean
    }
  >()

  for (const point of series) {
    const segment = Math.floor(point.ts / segmentSeconds) * segmentSeconds
    const bucket = buckets.get(segment) ?? {
      requests: 0,
      successes: 0,
      rateSum: 0,
      points: 0,
      hasData: false,
    }
    bucket.requests += point.request_count ?? 0
    bucket.successes += point.success_count ?? 0
    bucket.rateSum += point.success_rate
    bucket.points += 1
    bucket.hasData =
      bucket.hasData ||
      (point.request_count == null && Number.isFinite(point.success_rate)) ||
      (point.request_count ?? 0) > 0
    buckets.set(segment, bucket)
  }

  return Array.from(buckets.entries())
    .sort(([a], [b]) => a - b)
    .slice(-safeMaxSegments)
    .map(([ts, bucket]) => ({
      ts,
      hasData: bucket.hasData,
      successRate:
        bucket.requests > 0
          ? (bucket.successes / bucket.requests) * 100
          : bucket.rateSum / Math.max(1, bucket.points),
    }))
}
