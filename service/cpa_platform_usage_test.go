package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestReadCPAUsageSecretFromFile(t *testing.T) {
	secretFile := t.TempDir() + "/management-key"
	if err := os.WriteFile(secretFile, []byte("  secret-from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_CPA_SECRET", "")
	t.Setenv("TEST_CPA_SECRET_FILE", secretFile)
	got, err := readCPAUsageSecret("TEST_CPA_SECRET", "TEST_CPA_SECRET_FILE")
	if err != nil || got != "secret-from-file" {
		t.Fatalf("readCPAUsageSecret() = (%q, %v)", got, err)
	}
}

func TestLoadCPAUsageConfigsSupportsOptionalCPA2(t *testing.T) {
	for _, name := range []string{
		"CPA_USAGE_MANAGEMENT_URL",
		"CPA_USAGE_MANAGEMENT_KEY",
		"CPA_USAGE_MANAGEMENT_KEY_FILE",
		"CPA_USAGE_ANONYMIZATION_KEY",
		"CPA_USAGE_ANONYMIZATION_KEY_FILE",
		"CPA_USAGE_MANAGEMENT_URL_2",
		"CPA_USAGE_MANAGEMENT_KEY_2",
		"CPA_USAGE_MANAGEMENT_KEY_FILE_2",
		"CPA_USAGE_ANONYMIZATION_KEY_2",
		"CPA_USAGE_ANONYMIZATION_KEY_FILE_2",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("CPA_USAGE_MANAGEMENT_URL", "http://127.0.0.1:18096")
	t.Setenv("CPA_USAGE_MANAGEMENT_KEY", "cpa1-management")
	t.Setenv("CPA_USAGE_ANONYMIZATION_KEY", "cpa1-anonymize")
	t.Setenv("CPA_USAGE_MANAGEMENT_URL_2", "http://127.0.0.1:18097")
	t.Setenv("CPA_USAGE_MANAGEMENT_KEY_2", "cpa2-management")
	t.Setenv("CPA_USAGE_ANONYMIZATION_KEY_2", "cpa2-anonymize")

	configs, err := loadCPAUsageConfigs()
	if err != nil || len(configs) != 2 {
		t.Fatalf("loadCPAUsageConfigs() = (%+v, %v)", configs, err)
	}
	if configs[0].source != "cpa1" || configs[1].source != "cpa2" {
		t.Fatalf("unexpected CPA sources: %+v", configs)
	}
}

func TestMergeCPAModelUsageAggregatesCountersAndWeightedLatency(t *testing.T) {
	merged := mergeCPAModelUsage([]CPAModelUsage{
		{Model: "gpt-test", Provider: "codex", Requests: 2, Failed: 1, TotalTokens: 10, AvgLatencyMS: 100, AvgTTFTMS: 20, OutputTokensPerSecond: 5},
		{Model: "gpt-test", Provider: "codex", Requests: 1, Failed: 0, TotalTokens: 7, AvgLatencyMS: 400, AvgTTFTMS: 50, OutputTokensPerSecond: 11},
		{Model: "other", Provider: "codex", Requests: 1, TotalTokens: 99},
	})
	if len(merged) != 2 {
		t.Fatalf("merged model count = %d, want 2", len(merged))
	}
	if merged[0].Model != "gpt-test" || merged[0].Requests != 3 || merged[0].Failed != 1 || merged[0].TotalTokens != 17 {
		t.Fatalf("unexpected aggregate: %+v", merged[0])
	}
	if merged[0].AvgLatencyMS != 200 || merged[0].AvgTTFTMS != 30 || merged[0].OutputTokensPerSecond != 7 {
		t.Fatalf("unexpected weighted metrics: %+v", merged[0])
	}
}

func TestStableCPAAccountCodeIsKeyedAndStable(t *testing.T) {
	first := stableCPAAccountCode("account.json", "secret-one")
	if first != stableCPAAccountCode("account.json", "secret-one") {
		t.Fatal("account code must be stable")
	}
	if first == stableCPAAccountCode("account.json", "secret-two") {
		t.Fatal("account code must be keyed")
	}
}

func TestBuildCPAQuotaWindowsUsesRemainingPercentage(t *testing.T) {
	used := 97.0
	seconds := int64(604800)
	reset := int64(1234)
	windows := buildCPAQuotaWindows(cpaUsagePayload{RateLimit: &cpaRateLimitPayload{SecondaryWindow: &cpaUsageWindowPayload{UsedPercent: used, LimitWindowSeconds: seconds, ResetAt: reset}}})
	if len(windows) != 1 || windows[0].RemainingPercent == nil || *windows[0].RemainingPercent != 3 {
		t.Fatalf("unexpected windows: %+v", windows)
	}
}

func TestSortCPAAccountsByAvailabilityAndPrimaryRemaining(t *testing.T) {
	remaining := func(value float64) *float64 { return &value }
	accounts := []CPAAccountUsage{
		{Code: "A-LOW", Available: true, Enabled: true, Windows: []CPAQuotaWindow{{ID: "codex-primary", RemainingPercent: remaining(20)}}},
		{Code: "A-HIGH", Available: true, Enabled: true, Windows: []CPAQuotaWindow{{ID: "codex-primary", RemainingPercent: remaining(80)}}},
		{Code: "A-NO-QUOTA", Available: true, Enabled: true},
		{Code: "A-OFFLINE", Available: false, Enabled: true},
		{Code: "A-DISABLED", Available: false, Enabled: false, Windows: []CPAQuotaWindow{{ID: "codex-primary", RemainingPercent: remaining(100)}}},
	}

	sortCPAAccounts(accounts)
	got := make([]string, 0, len(accounts))
	for _, account := range accounts {
		got = append(got, account.Code)
	}
	want := []string{"A-HIGH", "A-LOW", "A-NO-QUOTA", "A-DISABLED", "A-OFFLINE"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sorted account codes = %v, want %v", got, want)
	}
}

func TestFindCPAFieldTraversesKnownWrappers(t *testing.T) {
	models := []interface{}{map[string]interface{}{"model": "gpt-test"}}
	got, ok := findCPAField(map[string]interface{}{"data": map[string]interface{}{"summary": map[string]interface{}{"models": models}}}, "models")
	if !ok || len(got.([]interface{})) != 1 {
		t.Fatalf("findCPAField() = (%v, %v)", got, ok)
	}
}

func TestFetchCPAAccountsSanitizesIdentityAndReadsQuotaViaFixedProxy(t *testing.T) {
	var apiCallSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer management-secret" {
			t.Fatalf("missing management authorization")
		}
		switch r.URL.Path {
		case "/auth-files":
			_, _ = w.Write([]byte(`{"files":[{"name":"codex-user@example.com-pro.json","type":"codex","email":"user@example.com","auth_index":"auth-1"}]}`))
		case "/api-call":
			apiCallSeen = true
			var requestBody map[string]interface{}
			if err := common.DecodeJson(r.Body, &requestBody); err != nil {
				t.Fatal(err)
			}
			if requestBody["method"] != http.MethodGet || requestBody["url"] != codexUsageURL {
				t.Fatalf("unexpected API call payload: %+v", requestBody)
			}
			_, _ = w.Write([]byte(`{"status_code":200,"body":{"plan_type":"pro","rate_limit":{"primary_window":{"used_percent":3,"limit_window_seconds":18000,"reset_at":1234}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	accounts, partial, err := fetchCPAAccounts(context.Background(), server.Client(), cpaUsageConfig{managementURL: server.URL, managementKey: "management-secret", anonymizeKey: "anonymous-secret", source: "cpa1"})
	if err != nil || partial {
		t.Fatalf("fetchCPAAccounts() error=%v partial=%v", err, partial)
	}
	if !apiCallSeen || len(accounts) != 1 || !accounts[0].Available {
		t.Fatalf("unexpected accounts: %+v", accounts)
	}
	if strings.Contains(accounts[0].Code, "user") {
		t.Fatalf("raw identity escaped sanitization: %+v", accounts[0])
	}
	payload, err := common.Marshal(accounts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "email") || strings.Contains(string(payload), "user@example.com") {
		t.Fatalf("email identity must not be serialized: %s", payload)
	}
	if strings.Contains(string(payload), "plan_type") {
		t.Fatalf("account plan type must not be serialized: %s", payload)
	}
	if !strings.Contains(string(payload), `"source":"cpa1"`) {
		t.Fatalf("account source should identify the sanitized instance: %s", payload)
	}
	if got := *accounts[0].Windows[0].RemainingPercent; got != 97 {
		t.Fatalf("remaining percent = %v, want 97", got)
	}
}
