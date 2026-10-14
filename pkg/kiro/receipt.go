// Package kiro implements the gateway side of the Kiro metering v1 contract.
// It never derives credits from tokens, text, or client-supplied data.
package kiro

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
)

const Unit = "kiro_credit"
const ContextKey = "kiro_metering_state"

var fixedPoint = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,9})?$`)

type Receipt struct {
	Version      int     `json:"version"`
	RequestID    string  `json:"request_id"`
	MeteringID   string  `json:"metering_id"`
	Unit         string  `json:"unit"`
	Billable     *string `json:"billable_credits,omitempty"`
	Consumed     *string `json:"consumed_credits,omitempty"`
	Status       string  `json:"status"`
	Coverage     string  `json:"consumed_coverage"`
	Source       string  `json:"source"`
	HopRequestID *string `json:"hlool_request_id,omitempty"`
	HopAttemptID *string `json:"hlool_attempt_id,omitempty"`
}

func Decimal(s string) (decimal.Decimal, error) {
	if !fixedPoint.MatchString(s) {
		return decimal.Zero, errors.New("expected a non-negative fixed-point decimal string (18 integer, 9 fractional digits)")
	}
	return decimal.NewFromString(s)
}

func ParseReceipt(raw []byte, requestID, attemptID string) (*Receipt, error) {
	if len(raw) > 8192 {
		return nil, errors.New("metering receipt too large")
	}
	var r Receipt
	if err := common.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("invalid metering receipt: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, errors.New("metering receipt must be an object")
	}
	for _, k := range []string{"billable_credits", "consumed_credits", "hlool_request_id", "hlool_attempt_id"} {
		if v, ok := fields[k]; ok && string(v) == "null" {
			return nil, fmt.Errorf("%s must be omitted or a string", k)
		}
	}
	if r.Version != 1 || r.Unit != Unit || r.Source != "upstream_metering" {
		return nil, errors.New("unsupported metering contract")
	}
	for _, id := range []string{r.RequestID, r.MeteringID} {
		if strings.TrimSpace(id) == "" || len(id) > 200 {
			return nil, errors.New("invalid metering identifier")
		}
	}
	if r.HopRequestID != nil && *r.HopRequestID != requestID {
		return nil, errors.New("metering request association mismatch")
	}
	if r.HopAttemptID != nil && *r.HopAttemptID != attemptID {
		return nil, errors.New("metering attempt association mismatch")
	}
	for _, s := range []*string{r.Billable, r.Consumed} {
		if s != nil {
			if _, err := Decimal(*s); err != nil {
				return nil, err
			}
		}
	}
	switch r.Status {
	case "complete":
		if r.Billable == nil || r.Coverage == "missing" {
			return nil, errors.New("complete receipt requires billable credits and known consumption")
		}
	case "incomplete":
		if r.Billable == nil {
			return nil, errors.New("incomplete receipt requires explicit zero billable credits")
		}
		b, _ := Decimal(*r.Billable)
		if !b.IsZero() {
			return nil, errors.New("incomplete receipt cannot bill credits")
		}
	case "missing":
		if r.Billable != nil {
			return nil, errors.New("missing receipt must omit billable credits")
		}
	default:
		return nil, errors.New("invalid metering status")
	}
	switch r.Coverage {
	case "complete", "partial":
		if r.Consumed == nil {
			return nil, errors.New("known coverage requires consumed credits")
		}
	case "missing":
		if r.Consumed != nil {
			return nil, errors.New("unknown coverage must omit consumed credits")
		}
	default:
		return nil, errors.New("invalid consumed coverage")
	}
	if r.Billable != nil && r.Consumed != nil {
		b, _ := Decimal(*r.Billable)
		c, _ := Decimal(*r.Consumed)
		if c.LessThan(b) {
			return nil, errors.New("consumed credits less than billable credits")
		}
	}
	return &r, nil
}

func MeteringKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

// canonical is only used for conflict detection. Forwarding retains the original
// decimal strings and metering_id, including all extension fields.
func (r Receipt) canonical() string {
	for _, p := range []**string{&r.Billable, &r.Consumed} {
		if *p != nil {
			d, _ := Decimal(**p)
			s := d.String()
			*p = &s
		}
	}
	b, _ := common.Marshal(r)
	return string(b)
}

func ForwardReceipt(raw []byte) json.RawMessage {
	var fields map[string]json.RawMessage
	if common.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil
	}
	delete(fields, "hlool_request_id")
	delete(fields, "hlool_attempt_id")
	b, _ := common.Marshal(fields)
	return b
}
