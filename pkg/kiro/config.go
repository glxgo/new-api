package kiro

import (
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
)

const OptionKey = "KiroCreditBilling"
const DefaultConfig = `{"mode":"off"}`

// Prices are platform-owned CNY values. Channel overrides affect procurement
// cost only; all eligible channels share the frozen retail price/conversion.
type Config struct {
	Mode             string         `json:"mode"`
	Groups           []string       `json:"groups"`
	Price            string         `json:"price_per_credit"`
	Cost             string         `json:"cost_per_credit"`
	QuotaPerCurrency int64          `json:"quota_per_currency"`
	Reserve          string         `json:"reserve_credits"`
	TrustedChannels  []int          `json:"trusted_channels"`
	ChannelCosts     map[int]string `json:"channel_cost_per_credit,omitempty"`
}

var current atomic.Pointer[Config]

func init() { current.Store(&Config{Mode: "off"}) }

func ParseConfig(value string) (*Config, error) {
	var c Config
	if err := common.UnmarshalJsonStr(value, &c); err != nil {
		return nil, err
	}
	if c.Mode == "off" {
		return &c, nil
	}
	if c.Mode != "shadow" && c.Mode != "charge" {
		return nil, errors.New("mode must be off, shadow or charge")
	}
	if len(c.Groups) == 0 || len(c.TrustedChannels) == 0 {
		return nil, errors.New("groups and trusted_channels must be explicitly configured")
	}
	for _, g := range c.Groups {
		if g == "" || g == "auto" {
			return nil, errors.New("configure concrete service groups")
		}
	}
	for _, id := range c.TrustedChannels {
		if id <= 0 {
			return nil, errors.New("invalid trusted channel")
		}
	}
	for _, s := range []string{c.Price, c.Reserve} {
		d, err := Decimal(s)
		if err != nil || !d.IsPositive() {
			return nil, errors.New("price and reserve must be positive fixed-point strings")
		}
	}
	if _, err := Decimal(c.Cost); err != nil {
		return nil, fmt.Errorf("cost_per_credit: %w", err)
	}
	for id, s := range c.ChannelCosts {
		if id <= 0 {
			return nil, errors.New("invalid cost channel")
		}
		if _, err := Decimal(s); err != nil {
			return nil, fmt.Errorf("channel cost: %w", err)
		}
	}
	if c.QuotaPerCurrency <= 0 {
		return nil, errors.New("quota_per_currency must be a positive integer")
	}
	return &c, nil
}

func Configure(value string) error {
	c, err := ParseConfig(value)
	if err != nil {
		return err
	}
	current.Store(c)
	return nil
}

type Snapshot struct {
	Mode             string          `json:"mode"`
	Group            string          `json:"group"`
	Price            decimal.Decimal `json:"price_per_credit"`
	GroupRatio       decimal.Decimal `json:"group_ratio"`
	EffectivePrice   decimal.Decimal `json:"effective_price_per_credit"`
	QuotaPerCurrency int64           `json:"quota_per_currency"`
	ReserveQuota     int             `json:"reserve_quota"`
	config           *Config
}

func Freeze(group string, groupRatio float64) (*Snapshot, error) {
	c := current.Load()
	s := &Snapshot{Mode: "off", Group: group, config: c}
	if c.Mode == "off" {
		return s, nil
	}
	match := false
	for _, g := range c.Groups {
		if g == group {
			match = true
			break
		}
	}
	if !match {
		return s, nil
	}
	ratio, err := decimal.NewFromString(strconv.FormatFloat(groupRatio, 'f', -1, 64))
	if err != nil || ratio.IsNegative() {
		return nil, errors.New("invalid effective group ratio")
	}
	s.Mode = c.Mode
	s.Price, _ = Decimal(c.Price)
	s.GroupRatio = ratio
	s.EffectivePrice = s.Price.Mul(ratio)
	s.QuotaPerCurrency = c.QuotaPerCurrency
	s.ReserveQuota, err = s.Quota(c.Reserve)
	if s.Mode == "shadow" {
		// Shadow must never block legacy billing because a comparison overflows.
		err = nil
	}
	return s, err
}

func (s *Snapshot) Quota(credits string) (int, error) {
	d, err := Decimal(credits)
	if err != nil {
		return 0, err
	}
	q := d.Mul(s.EffectivePrice).Mul(decimal.NewFromInt(s.QuotaPerCurrency)).Round(0)
	// ParseInt detects overflow before converting to the platform's int width.
	i, err := strconv.ParseInt(q.StringFixed(0), 10, strconv.IntSize)
	if err != nil || i < 0 {
		return 0, errors.New("credit quota overflow")
	}
	return int(i), nil
}

func (s *Snapshot) Trusted(channel int) bool {
	if s.Mode == "off" {
		return true
	} // validate/forward configured upstream data, without billing
	if s.config == nil {
		return false
	}
	for _, id := range s.config.TrustedChannels {
		if id == channel {
			return true
		}
	}
	return false
}

func (s *Snapshot) CostPrice(channel int) string {
	if s.config == nil {
		return ""
	}
	if v, ok := s.config.ChannelCosts[channel]; ok {
		return v
	}
	return s.config.Cost
}

func (s *Snapshot) PublicPrice() map[string]any {
	return map[string]any{"unit": Unit, "currency": "CNY", "price_per_credit": s.Price.String(), "group_ratio": s.GroupRatio.String(), "effective_price_per_credit": s.EffectivePrice.String()}
}
