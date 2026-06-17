package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// AffiliateSettleSetting T+1 分润结算配置(超管后台「运营设置」可配)。
type AffiliateSettleSetting struct {
	Enabled    bool `json:"enabled"`     // 是否启用 T+1 分润结算定时任务
	SettleHour int  `json:"settle_hour"` // 每日结算时刻(0-23, 本地时区), 默认 2
}

var affiliateSettleSetting = AffiliateSettleSetting{
	Enabled:    false, // 默认关闭, 超管确认 ModelCost/比例配好后再开
	SettleHour: 2,
}

func init() {
	config.GlobalConfig.Register("affiliate_settle_setting", &affiliateSettleSetting)
}

// GetAffiliateSettleSetting 获取 T+1 结算配置。
func GetAffiliateSettleSetting() *AffiliateSettleSetting {
	return &affiliateSettleSetting
}

// IsAffiliateSettleEnabled 是否启用 T+1 分润结算。
func IsAffiliateSettleEnabled() bool {
	return affiliateSettleSetting.Enabled
}

// GetAffiliateSettleHour 结算时刻(本地时区), 越界回退到默认 2。
func GetAffiliateSettleHour() int {
	if affiliateSettleSetting.SettleHour < 0 || affiliateSettleSetting.SettleHour > 23 {
		return 2
	}
	return affiliateSettleSetting.SettleHour
}
