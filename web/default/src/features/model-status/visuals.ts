export const HEALTHY_AVAILABILITY_THRESHOLD = 90
export const UNSTABLE_AVAILABILITY_THRESHOLD = 80

export function availabilityBarClass(successRate: number): string {
  if (successRate >= HEALTHY_AVAILABILITY_THRESHOLD) {
    return 'bg-emerald-500/90 dark:bg-emerald-400/85'
  }
  if (successRate >= UNSTABLE_AVAILABILITY_THRESHOLD) {
    return 'bg-amber-400/90 dark:bg-amber-300/80'
  }
  return 'bg-rose-500/90 dark:bg-rose-400/85'
}
