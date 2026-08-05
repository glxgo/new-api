package model

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	APIIngressCodeOptimized = "optimized"
	APIIngressCodeDirect    = "direct"
	APIIngressNetworkLine   = "line"
	APIIngressNetworkDirect = "direct"
	APIIngressPPMOne        = int64(1_000_000)
)

// APIIngressProfile describes one public API entrance. The reverse proxy is
// responsible for injecting the code before the request reaches the relay.
// BillingMultiplierPPM is stored as a fixed-point multiplier to avoid float
// drift in the hot billing path (950000 means 0.95x).
type APIIngressProfile struct {
	Id                   int    `json:"id"`
	Code                 string `json:"code" gorm:"uniqueIndex;type:varchar(64)"`
	DisplayName          string `json:"display_name" gorm:"type:varchar(128)"`
	PublicBaseURL        string `json:"public_base_url" gorm:"type:varchar(512)"`
	NetworkMode          string `json:"network_mode" gorm:"type:varchar(32)"`
	Description          string `json:"description" gorm:"type:varchar(255)"`
	BillingMultiplierPPM int64  `json:"billing_multiplier_ppm" gorm:"type:bigint;not null;default:1000000"`
	Enabled              bool   `json:"enabled" gorm:"not null;default:true;index"`
	Visible              bool   `json:"visible" gorm:"not null;default:true"`
	Default              bool   `json:"default" gorm:"not null;default:false;index"`
	ProbeEnabled         bool   `json:"probe_enabled" gorm:"not null;default:true"`
	SortOrder            int    `json:"sort_order" gorm:"not null;default:0"`
	CreatedAt            int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt            int64  `json:"updated_at" gorm:"bigint"`
}

func (p *APIIngressProfile) Normalize() {
	if p == nil {
		return
	}
	p.Code = strings.TrimSpace(strings.ToLower(p.Code))
	p.DisplayName = strings.TrimSpace(p.DisplayName)
	p.PublicBaseURL = strings.TrimRight(strings.TrimSpace(p.PublicBaseURL), "/")
	p.NetworkMode = strings.TrimSpace(strings.ToLower(p.NetworkMode))
	if p.NetworkMode == "" {
		p.NetworkMode = APIIngressNetworkDirect
	}
	if p.BillingMultiplierPPM <= 0 {
		p.BillingMultiplierPPM = APIIngressPPMOne
	}
	if p.BillingMultiplierPPM > 2_000_000 {
		p.BillingMultiplierPPM = 2_000_000
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = common.GetTimestamp()
	}
	p.UpdatedAt = common.GetTimestamp()
}

func (p *APIIngressProfile) Validate() error {
	if p == nil || p.Code == "" || len(p.Code) > 64 {
		return errors.New("API 入口编码不能为空且长度不能超过 64")
	}
	if strings.ContainsAny(p.Code, " /\\") {
		return errors.New("API 入口编码不能包含空格或路径分隔符")
	}
	if p.PublicBaseURL != "" {
		u, err := url.ParseRequestURI(p.PublicBaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return errors.New("API 入口地址必须是 http/https URL")
		}
	}
	return nil
}

var apiIngressCache struct {
	sync.RWMutex
	profiles map[string]*APIIngressProfile
}

func refreshAPIIngressCache(profiles []*APIIngressProfile) {
	cache := make(map[string]*APIIngressProfile, len(profiles))
	for _, profile := range profiles {
		if profile == nil || profile.Code == "" {
			continue
		}
		copy := *profile
		cache[copy.Code] = &copy
	}
	apiIngressCache.Lock()
	apiIngressCache.profiles = cache
	apiIngressCache.Unlock()
}

func loadAPIIngressCache() error {
	if DB == nil {
		return nil
	}
	var profiles []*APIIngressProfile
	if err := DB.Order("sort_order asc, id asc").Find(&profiles).Error; err != nil {
		return err
	}
	refreshAPIIngressCache(profiles)
	return nil
}

func GetAPIIngressProfileByCode(code string) (*APIIngressProfile, error) {
	code = strings.TrimSpace(strings.ToLower(code))
	apiIngressCache.RLock()
	profile := apiIngressCache.profiles[code]
	apiIngressCache.RUnlock()
	if profile != nil {
		copy := *profile
		return &copy, nil
	}
	if DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var result APIIngressProfile
	if err := DB.Where("code = ?", code).First(&result).Error; err != nil {
		return nil, err
	}
	refresh, err := ListAPIIngressProfiles(false)
	if err == nil {
		refreshAPIIngressCache(refresh)
	}
	return &result, nil
}

func GetDefaultAPIIngressProfile() (*APIIngressProfile, error) {
	apiIngressCache.RLock()
	for _, profile := range apiIngressCache.profiles {
		if profile.Default && profile.Enabled {
			copy := *profile
			apiIngressCache.RUnlock()
			return &copy, nil
		}
	}
	apiIngressCache.RUnlock()
	if DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var result APIIngressProfile
	err := DB.Where("enabled = ?", true).Order("default desc, sort_order asc, id asc").First(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAPIIngressProfileByHost lets deployments select an entrance without a
// custom header when both public URLs point at the same New API process.
func GetAPIIngressProfileByHost(host string) (*APIIngressProfile, error) {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	apiIngressCache.RLock()
	hasProfiles := len(apiIngressCache.profiles) > 0
	apiIngressCache.RUnlock()
	if !hasProfiles {
		_ = loadAPIIngressCache()
	}
	apiIngressCache.RLock()
	for _, profile := range apiIngressCache.profiles {
		if profile == nil || profile.PublicBaseURL == "" {
			continue
		}
		baseURL, err := url.Parse(profile.PublicBaseURL)
		if err == nil && strings.EqualFold(baseURL.Hostname(), host) {
			copy := *profile
			apiIngressCache.RUnlock()
			return &copy, nil
		}
	}
	apiIngressCache.RUnlock()
	return nil, gorm.ErrRecordNotFound
}

func ListAPIIngressProfiles(includeDisabled bool) ([]*APIIngressProfile, error) {
	if DB == nil {
		return []*APIIngressProfile{}, nil
	}
	query := DB.Order("sort_order asc, id asc")
	if !includeDisabled {
		query = query.Where("enabled = ? AND visible = ?", true, true)
	}
	var profiles []*APIIngressProfile
	if err := query.Find(&profiles).Error; err != nil {
		return nil, err
	}
	return profiles, nil
}

func SaveAPIIngressProfile(profile *APIIngressProfile) error {
	if profile == nil {
		return errors.New("API 入口不能为空")
	}
	profile.Normalize()
	if err := profile.Validate(); err != nil {
		return err
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if profile.Default {
			if err := tx.Model(&APIIngressProfile{}).Where("id <> ?", profile.Id).Update("default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(profile).Error
	})
	if err != nil {
		return err
	}
	return loadAPIIngressCache()
}

func EnsureDefaultAPIIngressProfiles() error {
	if DB == nil {
		return nil
	}
	var count int64
	if err := DB.Model(&APIIngressProfile{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		profiles := []*APIIngressProfile{
			{Code: APIIngressCodeOptimized, DisplayName: "三网优化 URL", NetworkMode: APIIngressNetworkLine, Description: "用户 → 线路机 → 落地机", BillingMultiplierPPM: APIIngressPPMOne, Enabled: true, Visible: true, Default: true, ProbeEnabled: true, SortOrder: 10},
			{Code: APIIngressCodeDirect, DisplayName: "海外直链 URL", NetworkMode: APIIngressNetworkDirect, Description: "用户直连落地机，可配置额外折扣", BillingMultiplierPPM: 950_000, Enabled: false, Visible: true, Default: false, ProbeEnabled: true, SortOrder: 20},
		}
		for _, profile := range profiles {
			profile.Normalize()
			if err := DB.Create(profile).Error; err != nil {
				return err
			}
		}
	}
	return loadAPIIngressCache()
}
