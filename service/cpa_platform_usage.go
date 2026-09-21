package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

const (
	cpaUsageRefreshInterval = 10 * time.Minute
	cpaUsageRequestTimeout  = 12 * time.Second
	cpaUsageMaxResponseSize = 4 << 20
	codexUsageURL           = "https://chatgpt.com/backend-api/wham/usage"
)

type CPAQuotaWindow struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	UsedPercent      *float64 `json:"used_percent"`
	RemainingPercent *float64 `json:"remaining_percent"`
	ResetAt          *int64   `json:"reset_at"`
	WindowSeconds    *int64   `json:"window_seconds"`
}

type CPAAccountUsage struct {
	Code string `json:"code"`
	// Source identifies the sanitized CPA instance that supplied this account.
	// It is intentionally a coarse label (for example cpa1/cpa2), never an
	// email, auth-file name or credential value.
	Source    string           `json:"source,omitempty"`
	PlanType  string           `json:"-"`
	Available bool             `json:"available"`
	Enabled   bool             `json:"enabled"`
	Windows   []CPAQuotaWindow `json:"windows"`
}

type CPAModelUsage struct {
	Model                 string  `json:"model"`
	Alias                 string  `json:"alias"`
	Provider              string  `json:"provider"`
	Requests              int64   `json:"requests"`
	Failed                int64   `json:"failed"`
	TotalTokens           int64   `json:"total_tokens"`
	InputTokens           int64   `json:"input_tokens"`
	OutputTokens          int64   `json:"output_tokens"`
	ReasoningTokens       int64   `json:"reasoning_tokens"`
	CachedTokens          int64   `json:"cached_tokens"`
	CacheReadTokens       int64   `json:"cache_read_tokens"`
	CacheCreationTokens   int64   `json:"cache_creation_tokens"`
	CostUSD               float64 `json:"cost_usd"`
	CostAvailable         bool    `json:"cost_available"`
	AvgLatencyMS          float64 `json:"avg_latency_ms"`
	AvgTTFTMS             float64 `json:"avg_ttft_ms"`
	OutputTokensPerSecond float64 `json:"output_tokens_per_second"`
	SlowRequests          int64   `json:"slow_requests"`
	SlowTTFTRequests      int64   `json:"slow_ttft_requests"`
}

type CPAUsageSnapshot struct {
	Configured    bool              `json:"configured"`
	Status        string            `json:"status"`
	UpdatedAt     int64             `json:"updated_at"`
	NextRefreshAt int64             `json:"next_refresh_at"`
	Accounts      []CPAAccountUsage `json:"accounts"`
	Models        []CPAModelUsage   `json:"models"`
}

type cpaUsageConfig struct {
	managementURL string
	managementKey string
	anonymizeKey  string
	source        string
}

type cpaAuthFile struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Provider    string      `json:"provider"`
	AuthIndex   interface{} `json:"auth_index"`
	Disabled    bool        `json:"disabled"`
	Unavailable bool        `json:"unavailable"`
	PlanType    string      `json:"plan_type"`
}

type cpaAuthFilesResponse struct {
	Files []cpaAuthFile `json:"files"`
}

type cpaAPICallResponse struct {
	StatusCode int         `json:"status_code"`
	Body       interface{} `json:"body"`
}

type cpaUsageWindowPayload struct {
	UsedPercent        interface{} `json:"used_percent"`
	LimitWindowSeconds interface{} `json:"limit_window_seconds"`
	ResetAt            interface{} `json:"reset_at"`
}

type cpaRateLimitPayload struct {
	PrimaryWindow   *cpaUsageWindowPayload `json:"primary_window"`
	SecondaryWindow *cpaUsageWindowPayload `json:"secondary_window"`
}

type cpaAdditionalRateLimitPayload struct {
	LimitName string               `json:"limit_name"`
	RateLimit *cpaRateLimitPayload `json:"rate_limit"`
}

type cpaUsagePayload struct {
	PlanType             string                          `json:"plan_type"`
	RateLimit            *cpaRateLimitPayload            `json:"rate_limit"`
	CodeReviewRateLimit  *cpaRateLimitPayload            `json:"code_review_rate_limit"`
	AdditionalRateLimits []cpaAdditionalRateLimitPayload `json:"additional_rate_limits"`
}

var (
	cpaUsageOnce    sync.Once
	cpaUsageRunning atomic.Bool
	cpaUsageMu      sync.RWMutex
	cpaUsageState   = CPAUsageSnapshot{Status: "unconfigured", Accounts: []CPAAccountUsage{}, Models: []CPAModelUsage{}}
)

func loadCPAUsageConfig() (cpaUsageConfig, error) {
	config, err, configured := loadCPAUsageConfigVariant("", "cpa1")
	if !configured && err == nil {
		return config, fmt.Errorf("CPA usage management URL or key is not configured")
	}
	return config, err
}

// loadCPAUsageConfigVariant reads one isolated CPA management configuration.
// The configured return value distinguishes an intentionally omitted second
// instance from a partially configured (and therefore invalid) instance.
func loadCPAUsageConfigVariant(suffix, source string) (cpaUsageConfig, error, bool) {
	urlEnv := "CPA_USAGE_MANAGEMENT_URL" + suffix
	managementKeyEnv := "CPA_USAGE_MANAGEMENT_KEY" + suffix
	managementKeyFileEnv := "CPA_USAGE_MANAGEMENT_KEY_FILE" + suffix
	anonymizeKeyEnv := "CPA_USAGE_ANONYMIZATION_KEY" + suffix
	anonymizeKeyFileEnv := "CPA_USAGE_ANONYMIZATION_KEY_FILE" + suffix
	configured := strings.TrimSpace(os.Getenv(urlEnv)) != "" ||
		strings.TrimSpace(os.Getenv(managementKeyEnv)) != "" ||
		strings.TrimSpace(os.Getenv(managementKeyFileEnv)) != "" ||
		strings.TrimSpace(os.Getenv(anonymizeKeyEnv)) != "" ||
		strings.TrimSpace(os.Getenv(anonymizeKeyFileEnv)) != ""

	managementKey, err := readCPAUsageSecret(managementKeyEnv, managementKeyFileEnv)
	if err != nil {
		return cpaUsageConfig{source: source}, err, configured
	}
	anonymizeKey, err := readCPAUsageSecret(anonymizeKeyEnv, anonymizeKeyFileEnv)
	if err != nil {
		return cpaUsageConfig{source: source}, err, configured
	}
	config := cpaUsageConfig{
		managementURL: strings.TrimRight(strings.TrimSpace(os.Getenv(urlEnv)), "/"),
		managementKey: managementKey,
		anonymizeKey:  anonymizeKey,
		source:        source,
	}
	if config.managementURL == "" || config.managementKey == "" {
		return config, fmt.Errorf("CPA usage management URL or key is not configured"), configured
	}
	parsed, err := url.Parse(config.managementURL)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return config, fmt.Errorf("CPA usage management URL must be a plain internal HTTP URL"), configured
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || (!ip.IsPrivate() && !ip.IsLoopback()) {
		return config, fmt.Errorf("CPA usage management URL must use a private or loopback IP"), configured
	}
	if config.anonymizeKey == "" {
		config.anonymizeKey = config.managementKey
	}
	return config, nil, configured
}

// loadCPAUsageConfigs supports CPA1 plus an optional isolated CPA2. A valid
// instance remains usable when the other instance is unavailable; callers can
// surface the partial state without failing the entire public pool.
func loadCPAUsageConfigs() ([]cpaUsageConfig, error) {
	variants := []struct {
		suffix string
		source string
	}{
		{suffix: "", source: "cpa1"},
		{suffix: "_2", source: "cpa2"},
	}
	configs := make([]cpaUsageConfig, 0, len(variants))
	var configErrors []string
	for _, variant := range variants {
		config, err, configured := loadCPAUsageConfigVariant(variant.suffix, variant.source)
		if !configured {
			continue
		}
		if err != nil {
			configErrors = append(configErrors, variant.source+": "+err.Error())
			continue
		}
		configs = append(configs, config)
	}
	if len(configs) == 0 {
		if len(configErrors) > 0 {
			return nil, fmt.Errorf("CPA usage configuration invalid: %s", strings.Join(configErrors, "; "))
		}
		return nil, fmt.Errorf("CPA usage management URL or key is not configured")
	}
	if len(configErrors) > 0 {
		return configs, fmt.Errorf("CPA usage configuration partial: %s", strings.Join(configErrors, "; "))
	}
	return configs, nil
}

func readCPAUsageSecret(valueEnv, fileEnv string) (string, error) {
	directValue := strings.TrimSpace(os.Getenv(valueEnv))
	filePath := strings.TrimSpace(os.Getenv(fileEnv))
	if directValue != "" && filePath != "" {
		return "", fmt.Errorf("%s and %s cannot both be configured", valueEnv, fileEnv)
	}
	if filePath == "" {
		return directValue, nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read CPA usage secret file: %w", err)
	}
	if len(data) > 4096 {
		return "", fmt.Errorf("CPA usage secret file is unexpectedly large")
	}
	return strings.TrimSpace(string(data)), nil
}

func StartCPAPlatformUsageTask() {
	cpaUsageOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		configs, configErr := loadCPAUsageConfigs()
		if len(configs) == 0 {
			logger.LogWarn(context.Background(), "CPA platform usage task disabled: required internal configuration is missing or invalid")
			return
		}
		cpaUsageMu.Lock()
		cpaUsageState.Configured = true
		if configErr != nil {
			cpaUsageState.Status = "partial"
		} else {
			cpaUsageState.Status = "syncing"
		}
		cpaUsageMu.Unlock()

		common.BackgroundCtxGo("report", context.Background(), func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("CPA platform usage task started: tick=%s instances=%d", cpaUsageRefreshInterval, len(configs)))
			runCPAPlatformUsageRefresh(configs, configErr != nil)
			ticker := time.NewTicker(cpaUsageRefreshInterval)
			defer ticker.Stop()
			for range ticker.C {
				runCPAPlatformUsageRefresh(configs, configErr != nil)
			}
		})
	})
}

func GetCPAUsageSnapshot() CPAUsageSnapshot {
	cpaUsageMu.RLock()
	defer cpaUsageMu.RUnlock()
	snapshot := cpaUsageState
	snapshot.Accounts = append([]CPAAccountUsage(nil), cpaUsageState.Accounts...)
	snapshot.Models = append([]CPAModelUsage(nil), cpaUsageState.Models...)
	return snapshot
}

func runCPAPlatformUsageRefresh(configs []cpaUsageConfig, configPartial bool) {
	if !common.BackgroundWorkAllowed() {
		logger.LogInfo(context.Background(), "CPA platform usage refresh deferred while system protection is active")
		return
	}
	if !cpaUsageRunning.CompareAndSwap(false, true) {
		return
	}
	defer cpaUsageRunning.Store(false)

	ctx, cancel := context.WithTimeout(context.Background(), cpaUsageRequestTimeout*time.Duration(4))
	defer cancel()
	client := &http.Client{Timeout: cpaUsageRequestTimeout}
	type instanceResult struct {
		accounts []CPAAccountUsage
		models   []CPAModelUsage
		partial  bool
		modelErr bool
		err      error
	}
	results := make(chan instanceResult, len(configs))
	for _, config := range configs {
		config := config
		go func() {
			accounts, accountPartial, err := fetchCPAAccounts(ctx, client, config)
			if err != nil {
				results <- instanceResult{partial: true, err: err}
				return
			}
			models, modelErr := fetchCPAModelUsage(ctx, client, config)
			results <- instanceResult{accounts: accounts, models: models, partial: accountPartial || modelErr != nil, modelErr: modelErr != nil, err: modelErr}
		}()
	}
	accounts := make([]CPAAccountUsage, 0)
	models := make([]CPAModelUsage, 0)
	accountSuccesses := 0
	partial := configPartial
	var modelErrors []string
	modelFailed := false
	for range configs {
		result := <-results
		if result.err != nil && len(result.accounts) == 0 {
			partial = true
			if result.modelErr {
				modelFailed = true
				modelErrors = append(modelErrors, result.err.Error())
			}
			continue
		}
		accountSuccesses++
		accounts = append(accounts, result.accounts...)
		models = append(models, result.models...)
		partial = partial || result.partial
		if result.modelErr {
			modelFailed = true
			modelErrors = append(modelErrors, result.err.Error())
		}
	}
	if accountSuccesses == 0 {
		markCPAUsageRefreshFailed()
		logger.LogWarn(context.Background(), "CPA platform usage refresh failed while reading account quotas")
		return
	}
	sortCPAAccounts(accounts)
	models = mergeCPAModelUsage(models)
	now := time.Now()

	cpaUsageMu.Lock()
	if modelFailed && len(cpaUsageState.Models) > 0 {
		models = cpaUsageState.Models
	}
	cpaUsageState = CPAUsageSnapshot{
		Configured: true,
		Status: func() string {
			if partial || len(modelErrors) > 0 {
				return "partial"
			}
			return "fresh"
		}(),
		UpdatedAt: now.Unix(), NextRefreshAt: now.Add(cpaUsageRefreshInterval).Unix(),
		Accounts: accounts, Models: models,
	}
	cpaUsageMu.Unlock()
	if len(modelErrors) > 0 {
		logger.LogWarn(context.Background(), "CPA platform usage model summary refresh failed; retained the previous model snapshot when available")
	}
}

func markCPAUsageRefreshFailed() {
	cpaUsageMu.Lock()
	defer cpaUsageMu.Unlock()
	if cpaUsageState.UpdatedAt > 0 {
		cpaUsageState.Status = "stale"
	} else {
		cpaUsageState.Status = "unavailable"
	}
	cpaUsageState.NextRefreshAt = time.Now().Add(cpaUsageRefreshInterval).Unix()
}

func doCPAManagementJSON(ctx context.Context, client *http.Client, config cpaUsageConfig, method, path string, body io.Reader, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, method, config.managementURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.managementKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("CPA management request returned HTTP %d", resp.StatusCode)
	}
	return common.DecodeJson(io.LimitReader(resp.Body, cpaUsageMaxResponseSize), target)
}

func fetchCPAAccounts(ctx context.Context, client *http.Client, config cpaUsageConfig) ([]CPAAccountUsage, bool, error) {
	var authFiles cpaAuthFilesResponse
	if err := doCPAManagementJSON(ctx, client, config, http.MethodGet, "/auth-files", nil, &authFiles); err != nil {
		return nil, false, err
	}
	files := make([]cpaAuthFile, 0, len(authFiles.Files))
	seen := make(map[string]bool)
	for _, file := range authFiles.Files {
		provider := strings.ToLower(strings.TrimSpace(file.Type + " " + file.Provider))
		if !strings.Contains(provider, "codex") || file.Name == "" || seen[file.Name] {
			continue
		}
		seen[file.Name] = true
		files = append(files, file)
	}

	accounts := make([]CPAAccountUsage, len(files))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 2)
	var partial atomic.Bool
	for index := range files {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			file := files[index]
			account := CPAAccountUsage{
				Code: stableCPAAccountCode(config.source+":"+file.Name, config.anonymizeKey), Source: config.source, PlanType: strings.ToLower(file.PlanType),
				Available: false, Enabled: !file.Disabled, Windows: []CPAQuotaWindow{},
			}
			if file.Disabled || file.Unavailable {
				accounts[index] = account
				return
			}
			semaphore <- struct{}{}
			payload, err := fetchCPAAccountQuota(ctx, client, config, file.AuthIndex)
			<-semaphore
			if err != nil {
				partial.Store(true)
				accounts[index] = account
				return
			}
			account.Available = true
			if payload.PlanType != "" {
				account.PlanType = strings.ToLower(payload.PlanType)
			}
			account.Windows = buildCPAQuotaWindows(payload)
			accounts[index] = account
		}()
	}
	wg.Wait()
	sortCPAAccounts(accounts)
	return accounts, partial.Load(), nil
}

// sortCPAAccounts keeps the public account pool useful at a glance: accounts
// with a fresh quota response are listed first, then by their primary quota
// window's remaining percentage (descending). Missing quota values sort below
// known values, with the anonymized code as a stable tie-breaker.
func sortCPAAccounts(accounts []CPAAccountUsage) {
	sort.SliceStable(accounts, func(i, j int) bool {
		if accounts[i].Available != accounts[j].Available {
			return accounts[i].Available
		}
		left, leftOK := cpaPrimaryRemainingPercent(accounts[i])
		right, rightOK := cpaPrimaryRemainingPercent(accounts[j])
		if leftOK != rightOK {
			return leftOK
		}
		if leftOK && left != right {
			return left > right
		}
		return accounts[i].Code < accounts[j].Code
	})
}

func cpaPrimaryRemainingPercent(account CPAAccountUsage) (float64, bool) {
	for _, window := range account.Windows {
		// buildCPAQuotaWindows emits the primary window before the secondary
		// window for each quota family. Prefer an explicit primary ID when a
		// snapshot was assembled by another caller.
		if strings.HasSuffix(strings.ToLower(window.ID), "-primary") {
			if window.RemainingPercent != nil && math.IsNaN(*window.RemainingPercent) == false && math.IsInf(*window.RemainingPercent, 0) == false {
				return *window.RemainingPercent, true
			}
			return 0, false
		}
	}
	if len(account.Windows) == 0 || account.Windows[0].RemainingPercent == nil {
		return 0, false
	}
	remaining := *account.Windows[0].RemainingPercent
	if math.IsNaN(remaining) || math.IsInf(remaining, 0) {
		return 0, false
	}
	return remaining, true
}

func fetchCPAAccountQuota(ctx context.Context, client *http.Client, config cpaUsageConfig, authIndex interface{}) (cpaUsagePayload, error) {
	var result cpaAPICallResponse
	payload := map[string]interface{}{
		"auth_index": authIndex,
		"method":     http.MethodGet,
		"url":        codexUsageURL,
		"header": map[string]string{
			"Authorization": "Bearer $TOKEN$",
			"Content-Type":  "application/json",
			"User-Agent":    "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal",
		},
	}
	encoded, err := common.Marshal(payload)
	if err != nil {
		return cpaUsagePayload{}, err
	}
	if err = doCPAManagementJSON(ctx, client, config, http.MethodPost, "/api-call", strings.NewReader(string(encoded)), &result); err != nil {
		return cpaUsagePayload{}, err
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		return cpaUsagePayload{}, fmt.Errorf("CPA quota proxy returned HTTP %d", result.StatusCode)
	}
	var usage cpaUsagePayload
	switch body := result.Body.(type) {
	case string:
		err = common.UnmarshalJsonStr(body, &usage)
	default:
		var encodedBody []byte
		encodedBody, err = common.Marshal(body)
		if err == nil {
			err = common.Unmarshal(encodedBody, &usage)
		}
	}
	return usage, err
}

func buildCPAQuotaWindows(payload cpaUsagePayload) []CPAQuotaWindow {
	windows := make([]CPAQuotaWindow, 0, 6)
	appendLimit := func(prefix string, limit *cpaRateLimitPayload) {
		if limit == nil {
			return
		}
		appendWindow := func(kind string, source *cpaUsageWindowPayload) {
			if source == nil {
				return
			}
			label := prefix
			windowSeconds := cpaInt64Pointer(source.LimitWindowSeconds)
			if windowSeconds != nil {
				switch *windowSeconds {
				case 18000:
					label += " 5 小时"
				case 604800:
					label += " 周"
				default:
					label += " 额度"
				}
			}
			usedPercent := cpaFloat64Pointer(source.UsedPercent)
			var remaining *float64
			if usedPercent != nil {
				value := 100 - *usedPercent
				if value < 0 {
					value = 0
				}
				if value > 100 {
					value = 100
				}
				remaining = &value
			}
			resetAt := cpaInt64Pointer(source.ResetAt)
			if resetAt != nil && *resetAt > 1_000_000_000_000 {
				value := *resetAt / 1000
				resetAt = &value
			}
			windows = append(windows, CPAQuotaWindow{ID: strings.ToLower(strings.ReplaceAll(prefix+"-"+kind, " ", "-")), Label: strings.TrimSpace(label), UsedPercent: usedPercent, RemainingPercent: remaining, ResetAt: resetAt, WindowSeconds: windowSeconds})
		}
		appendWindow("primary", limit.PrimaryWindow)
		appendWindow("secondary", limit.SecondaryWindow)
	}
	appendLimit("Codex", payload.RateLimit)
	appendLimit("Code Review", payload.CodeReviewRateLimit)
	for _, additional := range payload.AdditionalRateLimits {
		label := strings.TrimSpace(additional.LimitName)
		if label == "" {
			label = "Additional"
		}
		appendLimit(label, additional.RateLimit)
	}
	return windows
}

func fetchCPAModelUsage(ctx context.Context, client *http.Client, config cpaUsageConfig) ([]CPAModelUsage, error) {
	var raw interface{}
	if err := doCPAManagementJSON(ctx, client, config, http.MethodGet, "/plugins/codex-token-usage/summary?window=today&limit=50", nil, &raw); err != nil {
		return nil, err
	}
	modelsValue, ok := findCPAField(raw, "models")
	if !ok {
		return nil, fmt.Errorf("CPA model summary did not contain models")
	}
	encoded, err := common.Marshal(modelsValue)
	if err != nil {
		return nil, err
	}
	models := make([]CPAModelUsage, 0)
	if err = common.Unmarshal(encoded, &models); err != nil {
		return nil, err
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].Requests == models[j].Requests {
			return models[i].TotalTokens > models[j].TotalTokens
		}
		return models[i].Requests > models[j].Requests
	})
	return models, nil
}

// mergeCPAModelUsage combines the same model/provider row across CPA
// instances so the public traffic table remains an aggregate rather than a
// duplicate list. Latencies are weighted by request count; token and failure
// counters are summed exactly.
func mergeCPAModelUsage(models []CPAModelUsage) []CPAModelUsage {
	if len(models) < 2 {
		return models
	}
	type key struct{ model, alias, provider string }
	merged := make(map[key]CPAModelUsage, len(models))
	for _, item := range models {
		k := key{model: item.Model, alias: item.Alias, provider: item.Provider}
		current, exists := merged[k]
		if !exists {
			merged[k] = item
			continue
		}
		oldRequests := current.Requests
		newRequests := oldRequests + item.Requests
		if newRequests > 0 {
			oldWeight := oldRequests
			if oldWeight < 0 {
				oldWeight = 0
			}
			newWeight := item.Requests
			if newWeight < 0 {
				newWeight = 0
			}
			current.AvgLatencyMS = (current.AvgLatencyMS*float64(oldWeight) + item.AvgLatencyMS*float64(newWeight)) / float64(newRequests)
			current.AvgTTFTMS = (current.AvgTTFTMS*float64(oldWeight) + item.AvgTTFTMS*float64(newWeight)) / float64(newRequests)
			current.OutputTokensPerSecond = (current.OutputTokensPerSecond*float64(oldWeight) + item.OutputTokensPerSecond*float64(newWeight)) / float64(newRequests)
		}
		current.Requests = newRequests
		current.Failed += item.Failed
		current.TotalTokens += item.TotalTokens
		current.InputTokens += item.InputTokens
		current.OutputTokens += item.OutputTokens
		current.ReasoningTokens += item.ReasoningTokens
		current.CachedTokens += item.CachedTokens
		current.CacheReadTokens += item.CacheReadTokens
		current.CacheCreationTokens += item.CacheCreationTokens
		current.CostUSD += item.CostUSD
		current.CostAvailable = current.CostAvailable || item.CostAvailable
		current.SlowRequests += item.SlowRequests
		current.SlowTTFTRequests += item.SlowTTFTRequests
		merged[k] = current
	}
	result := make([]CPAModelUsage, 0, len(merged))
	for _, item := range merged {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Requests == result[j].Requests {
			return result[i].TotalTokens > result[j].TotalTokens
		}
		return result[i].Requests > result[j].Requests
	})
	return result
}

func findCPAField(value interface{}, key string) (interface{}, bool) {
	object, ok := value.(map[string]interface{})
	if !ok {
		return nil, false
	}
	if found, exists := object[key]; exists {
		return found, true
	}
	for _, nestedKey := range []string{"data", "summary", "result"} {
		if nested, exists := object[nestedKey]; exists {
			if found, foundOK := findCPAField(nested, key); foundOK {
				return found, true
			}
		}
	}
	return nil, false
}

func stableCPAAccountCode(identity, secret string) string {
	digest := hmac.New(sha256.New, []byte(secret))
	_, _ = digest.Write([]byte(identity))
	return strings.ToUpper(fmt.Sprintf("A-%x", digest.Sum(nil)[:3]))
}

func cpaFloat64Pointer(value interface{}) *float64 {
	parsed, ok := numberFromInterface(value)
	if !ok {
		return nil
	}
	return &parsed
}

func cpaInt64Pointer(value interface{}) *int64 {
	parsed, ok := numberFromInterface(value)
	if !ok {
		return nil
	}
	integer := int64(parsed)
	return &integer
}

func numberFromInterface(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}
