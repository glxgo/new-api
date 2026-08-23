package access_setting

import (
	"os"

	"github.com/QuantumNous/new-api/setting/config"
)

// GeoIP 未知/查询失败策略
const (
	GeoIPUnknownPolicyAllow = "allow"
	GeoIPUnknownPolicyDeny  = "deny"
)

// AccessPolicy 中国大陆 IP 网页访问限制策略（PRD v0.4）。
// 字段通过 config.GlobalConfig 以 access_policy.<field> 的形式持久化到 option 表。
type AccessPolicy struct {
	// BlockMainlandWebAccess 开启后，GeoIP=CN 的网页请求返回 451；API 不受影响。
	BlockMainlandWebAccess bool `json:"block_mainland_web_access"`
	// IncludeHkMoTW 固定为 false：只拦中国大陆，不拦港澳台（用户已确认）。
	IncludeHkMoTW bool `json:"include_hk_mo_tw"`
	// GeoIPUnknownPolicy 未知/查询失败策略：allow（放行+告警，默认）/ deny。
	GeoIPUnknownPolicy string `json:"geoip_unknown_policy"`
	// GeoIPDBPath GeoLite2-Country.mmdb 路径；为空时回退 GEOIP_COUNTRY_DB 环境变量或 /data/GeoLite2-Country.mmdb。
	GeoIPDBPath string `json:"geoip_db_path"`
	// GeoIPDBVersion 当前数据库版本标识（更新数据库时人工或脚本写入）。
	GeoIPDBVersion string `json:"geoip_db_version"`
	// ConfigVersion 策略配置版本号，每次修改 +1。
	ConfigVersion int64 `json:"config_version"`
}

var accessPolicy = AccessPolicy{
	BlockMainlandWebAccess: false,
	IncludeHkMoTW:          false,
	GeoIPUnknownPolicy:     GeoIPUnknownPolicyAllow,
	GeoIPDBPath:            "",
	GeoIPDBVersion:         "",
	ConfigVersion:          0,
}

func init() {
	config.GlobalConfig.Register("access_policy", &accessPolicy)
}

// GetAccessPolicy 返回全局访问策略（指针允许调用方只读访问）。
func GetAccessPolicy() *AccessPolicy {
	return &accessPolicy
}

// IsBlockMainlandWebAccess 是否开启中国大陆网页访问限制。
func IsBlockMainlandWebAccess() bool {
	return accessPolicy.BlockMainlandWebAccess
}

// UnknownPolicyAllows 未知 IP 是否放行（fail-open）。
func UnknownPolicyAllows() bool {
	return accessPolicy.GeoIPUnknownPolicy != GeoIPUnknownPolicyDeny
}

// GetGeoIPDBPath 解析实际使用的 GeoIP 数据库路径。
func GetGeoIPDBPath() string {
	if accessPolicy.GeoIPDBPath != "" {
		return accessPolicy.GeoIPDBPath
	}
	if p := os.Getenv("GEOIP_COUNTRY_DB"); p != "" {
		return p
	}
	return "/data/GeoLite2-Country.mmdb"
}
