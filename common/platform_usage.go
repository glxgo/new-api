package common

import "math"

const platformBaseQuotaKey = "platform_base_quota"

// PlatformBaseQuota removes customer-facing entrance and group multipliers
// while deliberately retaining model pricing and the priority service-tier
// surcharge. The boolean reports whether the group multiplier was usable.
func PlatformBaseQuota(finalQuota int64, groupRatio float64, ingressOriginalQuota int64) (int64, bool) {
	quotaBeforeIngress := ingressOriginalQuota
	if quotaBeforeIngress <= 0 {
		quotaBeforeIngress = finalQuota
	}
	if groupRatio <= 0 || math.IsNaN(groupRatio) || math.IsInf(groupRatio, 0) {
		return quotaBeforeIngress, false
	}
	return int64(math.Round(float64(quotaBeforeIngress) / groupRatio)), true
}

func PlatformBaseQuotaKey() string {
	return platformBaseQuotaKey
}
