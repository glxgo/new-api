export function detectSingleGroupRename(
  previousRaw: string,
  nextRaw: string
): Record<string, string> {
  try {
    const previous = JSON.parse(previousRaw) as Record<string, number>
    const next = JSON.parse(nextRaw) as Record<string, number>
    const removed = Object.keys(previous).filter((name) => !(name in next))
    const added = Object.keys(next).filter((name) => !(name in previous))
    if (removed.length === 1 && added.length === 1) {
      return { [removed[0]]: added[0] }
    }
  } catch {
    // Form validation owns malformed JSON errors; do not infer a rename.
  }
  return {}
}
