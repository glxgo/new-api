export function availabilityBarClass(successRate: number): string {
  return successRate >= 99
    ? 'bg-emerald-500/90 dark:bg-emerald-400/85'
    : 'bg-amber-400/90 dark:bg-amber-300/80'
}
