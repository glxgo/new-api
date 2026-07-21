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
}

export function resolveModelStatusGroups(
  data: GroupSummaryAllData['data'] | undefined,
  pricingGroups: string[] = []
) {
  return Array.from(
    new Set([
      ...(data?.available_groups ?? []),
      ...(data?.groups ?? []).map((summary) => summary.group),
      ...pricingGroups,
    ])
  ).filter(Boolean)
}

export function hasCompleteHealthMetrics(
  summary: GroupCacheSummary | undefined
) {
  return Number.isFinite(summary?.success_rate)
}

export function summarizeAvailabilitySeries(
  series: PerformanceSeriesPoint[],
  hours: number,
  maxSegments = 24
): AvailabilitySegment[] {
  const safeHours = Math.max(1, hours)
  const safeMaxSegments = Math.max(1, maxSegments)
  const segmentSeconds =
    Math.max(1, Math.ceil(safeHours / safeMaxSegments)) * 3600
  const buckets = new Map<
    number,
    { requests: number; successes: number; rateSum: number; points: number }
  >()

  for (const point of series) {
    const segment = Math.floor(point.ts / segmentSeconds) * segmentSeconds
    const bucket = buckets.get(segment) ?? {
      requests: 0,
      successes: 0,
      rateSum: 0,
      points: 0,
    }
    bucket.requests += point.request_count ?? 0
    bucket.successes += point.success_count ?? 0
    bucket.rateSum += point.success_rate
    bucket.points += 1
    buckets.set(segment, bucket)
  }

  return Array.from(buckets.entries())
    .sort(([a], [b]) => a - b)
    .slice(-safeMaxSegments)
    .map(([ts, bucket]) => ({
      ts,
      successRate:
        bucket.requests > 0
          ? (bucket.successes / bucket.requests) * 100
          : bucket.rateSum / Math.max(1, bucket.points),
    }))
}
