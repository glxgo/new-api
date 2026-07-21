/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

type RpmProfile = {
  concurrency_limit?: number | null
  rpm_limit?: number | null
  current_rpm?: number | null
}

export function resolveRpmLimit(profile: RpmProfile | null | undefined) {
  if (Number.isFinite(profile?.rpm_limit) && (profile?.rpm_limit ?? 0) > 0) {
    return Math.ceil(profile!.rpm_limit!)
  }

  const concurrency =
    Number.isFinite(profile?.concurrency_limit) &&
    (profile?.concurrency_limit ?? 0) > 0
      ? profile!.concurrency_limit!
      : 8
  return Math.ceil(concurrency * 1.5)
}

export function resolveCurrentRpm(
  profile: RpmProfile | null | undefined
): number | null {
  return Number.isFinite(profile?.current_rpm) &&
    (profile?.current_rpm ?? -1) >= 0
    ? Math.floor(profile!.current_rpm!)
    : null
}
