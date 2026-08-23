package commission_setting

import (
	"fmt"
	"strconv"
	"strings"
)

// Rates are basis points (10000 = 100%). They are persisted as ordinary
// options so existing deployments can change policy without a migration.
const (
	DefaultOrdinaryDirectBP   int64 = 500
	DefaultOrdinaryIndirectBP int64 = 200
	DefaultAgentDirectBP      int64 = 800
	DefaultAgentIndirectBP    int64 = 200
	DefaultAdminDirectBP      int64 = 1500
	DefaultAdminIndirectBP    int64 = 500
	DefaultRootBP             int64 = 500
)

var (
	OrdinaryDirectBP   = DefaultOrdinaryDirectBP
	OrdinaryIndirectBP = DefaultOrdinaryIndirectBP
	AgentDirectBP      = DefaultAgentDirectBP
	AgentIndirectBP    = DefaultAgentIndirectBP
	AdminDirectBP      = DefaultAdminDirectBP
	AdminIndirectBP    = DefaultAdminIndirectBP
	RootBP             = DefaultRootBP
)

const (
	KeyOrdinaryDirect   = "RechargeCommissionOrdinaryDirectBP"
	KeyOrdinaryIndirect = "RechargeCommissionOrdinaryIndirectBP"
	KeyAgentDirect      = "RechargeCommissionAgentDirectBP"
	KeyAgentIndirect    = "RechargeCommissionAgentIndirectBP"
	KeyAdminDirect      = "RechargeCommissionAdminDirectBP"
	KeyAdminIndirect    = "RechargeCommissionAdminIndirectBP"
	KeyRoot             = "RechargeCommissionRootBP"
)

func ParseBasisPoints(value string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 || parsed > 10000 {
		return 0, fmt.Errorf("commission rate must be between 0 and 10000 basis points")
	}
	return parsed, nil
}

func Apply(key, value string) error {
	bp, err := ParseBasisPoints(value)
	if err != nil {
		return err
	}
	switch key {
	case KeyOrdinaryDirect:
		OrdinaryDirectBP = bp
	case KeyOrdinaryIndirect:
		OrdinaryIndirectBP = bp
	case KeyAgentDirect:
		AgentDirectBP = bp
	case KeyAgentIndirect:
		AgentIndirectBP = bp
	case KeyAdminDirect:
		AdminDirectBP = bp
	case KeyAdminIndirect:
		AdminIndirectBP = bp
	case KeyRoot:
		RootBP = bp
	default:
		return fmt.Errorf("unknown commission setting %q", key)
	}
	return nil
}

func Values() map[string]int64 {
	return map[string]int64{
		KeyOrdinaryDirect:   OrdinaryDirectBP,
		KeyOrdinaryIndirect: OrdinaryIndirectBP,
		KeyAgentDirect:      AgentDirectBP,
		KeyAgentIndirect:    AgentIndirectBP,
		KeyAdminDirect:      AdminDirectBP,
		KeyAdminIndirect:    AdminIndirectBP,
		KeyRoot:             RootBP,
	}
}
