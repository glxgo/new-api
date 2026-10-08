package kiro

import (
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// Synthetic contract fixtures; none of these values came from Kiro.
const fixture = `{"version":1,"request_id":"producer-fixture","metering_id":"fixture-1","unit":"kiro_credit","billable_credits":"5.508301567","consumed_credits":"5.508301567","status":"complete","consumed_coverage":"complete","source":"upstream_metering"}`
const testConfig = `{"mode":"charge","groups":["kiro"],"price_per_credit":"0.11","cost_per_credit":"0.085","quota_per_currency":500000,"reserve_credits":"10","trusted_channels":[1,2],"channel_cost_per_credit":{"2":"0.05"}}`

func stateForTest(t *testing.T) *State {
	t.Helper()
	require.NoError(t, Configure(testConfig))
	t.Cleanup(func() { _ = Configure(DefaultConfig) })
	snap, err := Freeze("kiro", 1)
	require.NoError(t, err)
	s := NewState(snap)
	s.Begin(1)
	return s
}

func TestKiroReceiptContract(t *testing.T) {
	for _, tc := range []struct {
		name, old, value string
		valid            bool
	}{
		{"valid", "", "", true},
		{"zero", `"5.508301567"`, `"0"`, true},
		{"json_number", `"billable_credits":"5.508301567"`, `"billable_credits":5`, false},
		{"null", `"billable_credits":"5.508301567"`, `"billable_credits":null`, false},
		{"missing", `"billable_credits":"5.508301567",`, ``, false},
		{"negative", `"billable_credits":"5.508301567"`, `"billable_credits":"-1"`, false},
		{"exponent", `"billable_credits":"5.508301567"`, `"billable_credits":"1e0"`, false},
		{"ten_decimals", `"billable_credits":"5.508301567"`, `"billable_credits":"0.1234567890"`, false},
		{"nineteen_integers", `"billable_credits":"5.508301567"`, `"billable_credits":"1234567890123456789"`, false},
		{"consumed_less", `"consumed_credits":"5.508301567"`, `"consumed_credits":"1"`, false},
		{"partial", `"consumed_coverage":"complete"`, `"consumed_coverage":"partial"`, true},
		{"incomplete_nonzero", `"status":"complete"`, `"status":"incomplete"`, false},
		{"unknown_complete", `"consumed_coverage":"complete"`, `"consumed_coverage":"missing"`, false},
		{"wrong_unit", `"unit":"kiro_credit"`, `"unit":"CNY"`, false},
		{"wrong_source", `"source":"upstream_metering"`, `"source":"model"`, false},
		{"empty_id", `"fixture-1"`, `""`, false},
		{"long_id", `"fixture-1"`, `"` + strings.Repeat("x", 201) + `"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := fixture
			if tc.old != "" {
				raw = strings.ReplaceAll(raw, tc.old, tc.value)
			}
			_, err := ParseReceipt([]byte(raw), "hop", "attempt")
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	unknown := `{"version":1,"request_id":"p","metering_id":"m","unit":"kiro_credit","status":"missing","consumed_coverage":"missing","source":"upstream_metering"}`
	r, err := ParseReceipt([]byte(unknown), "", "")
	require.NoError(t, err)
	require.Nil(t, r.Billable)
	require.Nil(t, r.Consumed)
	incomplete := strings.Replace(unknown, `"status":"missing"`, `"status":"incomplete","billable_credits":"0"`, 1)
	_, err = ParseReceipt([]byte(incomplete), "", "")
	require.NoError(t, err)
	for _, bad := range []string{"NaN", "Infinity", "+1", ".1", "1.", " 0"} {
		_, err := Decimal(bad)
		require.Error(t, err)
	}
}

func TestKiroSnapshotPrecisionAndFreeze(t *testing.T) {
	s := stateForTest(t)
	q, err := s.Snapshot.Quota("5.508301567")
	require.NoError(t, err)
	require.Equal(t, 302957, q)
	require.Equal(t, 550000, s.Snapshot.ReserveQuota)
	q, err = s.Snapshot.Quota("0.0001")
	require.NoError(t, err)
	require.Equal(t, 6, q) // 5.5 HALF_UP
	q, err = s.Snapshot.Quota("0")
	require.NoError(t, err)
	require.Zero(t, q)
	q, err = s.Snapshot.Quota("20")
	require.NoError(t, err)
	require.Equal(t, 1100000, q) // reserve is not a cap
	_, err = s.Snapshot.Quota("999999999999999999.999999999")
	require.Error(t, err)
	require.NoError(t, Configure(strings.Replace(testConfig, `"0.11"`, `"1"`, 1)))
	q, err = s.Snapshot.Quota("1")
	require.NoError(t, err)
	require.Equal(t, 55000, q)
	require.Equal(t, "0.05", s.Snapshot.CostPrice(2))
	require.NotContains(t, s.Snapshot.PublicPrice(), "cost_per_credit")
}

func TestKiroConfigRejectsInvalid(t *testing.T) {
	for _, pair := range [][2]string{{`"0.11"`, `"0"`}, {`"10"`, `"0"`}, {`500000`, `0`}, {`500000`, `1.5`}, {`"0.085"`, `"-1"`}, {`[1,2]`, `[]`}, {`["kiro"]`, `["auto"]`}, {`"charge"`, `"bad"`}} {
		_, err := ParseConfig(strings.Replace(testConfig, pair[0], pair[1], 1))
		require.Error(t, err)
	}
	_, err := ParseConfig(strings.Replace(testConfig, `"0.085"`, `"0"`, 1))
	require.NoError(t, err)
	_, err = ParseConfig(DefaultConfig)
	require.NoError(t, err)
}

func TestKiroObserveAssociationsConflictRetryAndSpoof(t *testing.T) {
	s := stateForTest(t)
	r := s.Result()
	var receipt map[string]any
	require.NoError(t, common.Unmarshal([]byte(fixture), &receipt))
	receipt["hlool_request_id"] = r.RequestID
	receipt["hlool_attempt_id"] = r.AttemptID
	raw, _ := common.Marshal(receipt)
	payload := `{"usage":{"total_tokens":42,"kiro_metering":` + string(raw) + `}}`
	out := s.Observe([]byte(payload), false)
	require.NotContains(t, string(out), "hlool_")
	require.Contains(t, string(s.Result().Raw), r.AttemptID)
	s.Observe([]byte(payload), false)
	require.Empty(t, s.Result().Error)
	s.Observe([]byte(strings.Replace(payload, "fixture-1", "different", 1)), false)
	require.True(t, s.Result().MeteringInvalid)
	require.Nil(t, s.Forward())
	next := s.Begin(2)
	require.NotEqual(t, r.AttemptID, next.AttemptID)
	require.Equal(t, r.RequestID, next.RequestID)
	require.Nil(t, next.Receipt)
	require.Empty(t, next.Error)
	require.Equal(t, "0.05", next.CostPrice)
	s.Observe([]byte(payload), false)
	require.True(t, s.Result().MeteringInvalid) // stale hop IDs
	s.Begin(1)
	s.Observe([]byte(`{"choices":[{"message":{"content":"kiro_metering billable_credits 9999"}}]}`), false)
	require.Nil(t, s.Result().Receipt)
	s.Begin(99)
	s.Observe([]byte(`{"usage":{"kiro_metering":`+fixture+`}}`), false)
	require.True(t, s.Result().MeteringInvalid)
}

func TestKiroStateConcurrentReads(t *testing.T) {
	s := stateForTest(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Observe([]byte(`{"usage":{"kiro_metering":`+fixture+`}}`), true)
			_ = s.Result()
			_ = s.Forward()
		}()
	}
	wg.Wait()
	require.Empty(t, s.Result().Error)
}
