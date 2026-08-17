package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const (
	routeFailureWindow = 60 * time.Second
	routeOpenDuration  = 30 * time.Second
	routeFailureLimit  = 3
)

var localRouteCircuit = struct {
	sync.Mutex
	failures    map[string][]time.Time
	open        map[string]time.Time
	quota       map[string]time.Time
	quotaAmount map[string]int64
	walletOrder map[string][]int
}{failures: map[string][]time.Time{}, open: map[string]time.Time{}, quota: map[string]time.Time{}, quotaAmount: map[string]int64{}, walletOrder: map[string][]int{}}

const routeRuntimeTTL = 30 * 24 * time.Hour

func tokenRouteRuntimeSuffix(modelName string, relayFormat types.RelayFormat) string {
	replacer := strings.NewReplacer(" ", "_", ":", "_", "/", "_")
	return replacer.Replace(modelName) + ":" + string(relayFormat)
}

func walletRouteOrderKey(tokenId int, modelName string, relayFormat types.RelayFormat) string {
	return fmt.Sprintf("token-route-wallet-order:%d:%s", tokenId, tokenRouteRuntimeSuffix(modelName, relayFormat))
}

func walletRouteFailureKey(tokenId, stepId int, modelName string, relayFormat types.RelayFormat) string {
	return fmt.Sprintf("token-route-wallet-fail:%d:%d:%s", tokenId, stepId, tokenRouteRuntimeSuffix(modelName, relayFormat))
}

func encodeRouteStepIds(ids []int) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}

func decodeRouteStepIds(value string) []int {
	parts := strings.Split(value, ",")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func loadWalletRouteOrder(key string) []int {
	if common.RedisEnabled && common.RDB != nil {
		value, err := common.RDB.Get(context.Background(), key).Result()
		if err == nil {
			_ = common.RDB.Expire(context.Background(), key, routeRuntimeTTL).Err()
			return decodeRouteStepIds(value)
		}
		return nil
	}
	localRouteCircuit.Lock()
	defer localRouteCircuit.Unlock()
	return append([]int(nil), localRouteCircuit.walletOrder[key]...)
}

func storeWalletRouteOrder(key string, ids []int) {
	if common.RedisEnabled && common.RDB != nil {
		_ = common.RDB.Set(context.Background(), key, encodeRouteStepIds(ids), routeRuntimeTTL).Err()
		return
	}
	localRouteCircuit.Lock()
	localRouteCircuit.walletOrder[key] = append([]int(nil), ids...)
	localRouteCircuit.Unlock()
}

func normalizedWalletRouteOrder(key string, steps []model.TokenRouteStep) []int {
	configured := make([]int, 0, len(steps))
	valid := make(map[int]struct{}, len(steps))
	for _, step := range steps {
		if step.FundingSource == model.TokenRouteSourceWallet {
			configured = append(configured, step.Id)
			valid[step.Id] = struct{}{}
		}
	}
	stored := loadWalletRouteOrder(key)
	ordered := make([]int, 0, len(configured))
	seen := make(map[int]struct{}, len(configured))
	for _, id := range stored {
		if _, ok := valid[id]; ok {
			if _, duplicate := seen[id]; !duplicate {
				ordered = append(ordered, id)
				seen[id] = struct{}{}
			}
		}
	}
	for _, id := range configured {
		if _, ok := seen[id]; !ok {
			ordered = append(ordered, id)
		}
	}
	if encodeRouteStepIds(ordered) != encodeRouteStepIds(stored) {
		storeWalletRouteOrder(key, ordered)
	}
	return ordered
}

// orderTokenRouteSteps only permutes wallet steps. Entitlement steps keep
// their configured slots, while demoted wallet groups fill the last wallet
// slot and the next wallet group advances into the vacated slot.
func orderTokenRouteSteps(tokenId int, modelName string, relayFormat types.RelayFormat, steps []model.TokenRouteStep) []model.TokenRouteStep {
	result := append([]model.TokenRouteStep(nil), steps...)
	key := walletRouteOrderKey(tokenId, modelName, relayFormat)
	orderedIds := normalizedWalletRouteOrder(key, steps)
	byId := make(map[int]model.TokenRouteStep, len(orderedIds))
	for _, step := range steps {
		if step.FundingSource == model.TokenRouteSourceWallet {
			byId[step.Id] = step
		}
	}
	next := 0
	for index := range result {
		if result[index].FundingSource != model.TokenRouteSourceWallet || next >= len(orderedIds) {
			continue
		}
		result[index] = byId[orderedIds[next]]
		next++
	}
	return result
}

func demoteWalletRouteStep(tokenId, stepId int, modelName string, relayFormat types.RelayFormat) bool {
	key := walletRouteOrderKey(tokenId, modelName, relayFormat)
	ids := loadWalletRouteOrder(key)
	index := -1
	for i, id := range ids {
		if id == stepId {
			index = i
			break
		}
	}
	if index < 0 || len(ids) < 2 || index == len(ids)-1 {
		return false
	}
	ids = append(append(ids[:index:index], ids[index+1:]...), stepId)
	storeWalletRouteOrder(key, ids)
	return true
}

func recordWalletRouteUnavailable(tokenId, stepId int, modelName string, relayFormat types.RelayFormat) bool {
	key := walletRouteFailureKey(tokenId, stepId, modelName, relayFormat)
	if common.RedisEnabled && common.RDB != nil {
		ctx := context.Background()
		count, err := common.RDB.Incr(ctx, key).Result()
		if err != nil {
			return false
		}
		if count == 1 {
			_ = common.RDB.Expire(ctx, key, routeRuntimeTTL).Err()
		} else {
			_ = common.RDB.Expire(ctx, key, routeRuntimeTTL).Err()
		}
		if count < routeFailureLimit {
			return false
		}
		_ = common.RDB.Del(ctx, key).Err()
		return demoteWalletRouteStep(tokenId, stepId, modelName, relayFormat)
	}
	localRouteCircuit.Lock()
	window := append(localRouteCircuit.failures[key], time.Now())
	if len(window) < routeFailureLimit {
		localRouteCircuit.failures[key] = window
		localRouteCircuit.Unlock()
		return false
	}
	delete(localRouteCircuit.failures, key)
	localRouteCircuit.Unlock()
	return demoteWalletRouteStep(tokenId, stepId, modelName, relayFormat)
}

func clearWalletRouteFailures(tokenId, stepId int, modelName string, relayFormat types.RelayFormat) {
	key := walletRouteFailureKey(tokenId, stepId, modelName, relayFormat)
	if common.RedisEnabled && common.RDB != nil {
		_ = common.RDB.Del(context.Background(), key).Err()
		return
	}
	localRouteCircuit.Lock()
	delete(localRouteCircuit.failures, key)
	localRouteCircuit.Unlock()
}

func resetLocalTokenRouteRuntimeForTest() {
	localRouteCircuit.Lock()
	localRouteCircuit.failures = map[string][]time.Time{}
	localRouteCircuit.open = map[string]time.Time{}
	localRouteCircuit.quota = map[string]time.Time{}
	localRouteCircuit.quotaAmount = map[string]int64{}
	localRouteCircuit.walletOrder = map[string][]int{}
	localRouteCircuit.Unlock()
}

func tokenRouteQuotaKey(tokenId, stepId int) string {
	return fmt.Sprintf("token-route-quota:%d:%d", tokenId, stepId)
}

func tokenRouteQuotaUntil(tokenId, stepId int) time.Time {
	key := tokenRouteQuotaKey(tokenId, stepId)
	if common.RedisEnabled && common.RDB != nil {
		value, err := common.RDB.Get(context.Background(), key).Int64()
		if err == nil && value > time.Now().Unix() {
			return time.Unix(value, 0)
		}
		return time.Time{}
	}
	localRouteCircuit.Lock()
	defer localRouteCircuit.Unlock()
	until := localRouteCircuit.quota[key]
	if !until.IsZero() && time.Now().Before(until) {
		return until
	}
	delete(localRouteCircuit.quota, key)
	delete(localRouteCircuit.quotaAmount, key)
	return time.Time{}
}

func normalizeTokenRouteRequiredAmount(requiredAmount int64) int64 {
	if requiredAmount <= 0 {
		return 1
	}
	return requiredAmount
}

func tokenRouteQuotaRequiredAmount(tokenId, stepId int) int64 {
	key := tokenRouteQuotaKey(tokenId, stepId)
	if common.RedisEnabled && common.RDB != nil {
		amount, err := common.RDB.Get(context.Background(), key+":amount").Int64()
		if err == nil && amount > 0 {
			return amount
		}
		return 1
	}
	localRouteCircuit.Lock()
	defer localRouteCircuit.Unlock()
	return normalizeTokenRouteRequiredAmount(localRouteCircuit.quotaAmount[key])
}

func clearTokenRouteQuotaFreeze(tokenId, stepId int) {
	key := tokenRouteQuotaKey(tokenId, stepId)
	if common.RedisEnabled && common.RDB != nil {
		_ = common.RDB.Del(context.Background(), key, key+":amount").Err()
		return
	}
	localRouteCircuit.Lock()
	delete(localRouteCircuit.quota, key)
	delete(localRouteCircuit.quotaAmount, key)
	localRouteCircuit.Unlock()
}

func freezeTokenRouteQuota(tokenId, stepId int, resetAt, requiredAmount int64) {
	now := time.Now()
	until := time.Unix(resetAt, 0)
	if resetAt <= now.Unix() {
		until = now.Add(routeOpenDuration)
	}
	key := tokenRouteQuotaKey(tokenId, stepId)
	if common.RedisEnabled && common.RDB != nil {
		ttl := time.Until(until)
		_ = common.RDB.Set(context.Background(), key, until.Unix(), ttl).Err()
		_ = common.RDB.Set(context.Background(), key+":amount", normalizeTokenRouteRequiredAmount(requiredAmount), ttl).Err()
		return
	}
	localRouteCircuit.Lock()
	localRouteCircuit.quota[key] = until
	localRouteCircuit.quotaAmount[key] = normalizeTokenRouteRequiredAmount(requiredAmount)
	localRouteCircuit.Unlock()
}

func tokenRouteQuotaCooling(tokenId, stepId int) bool {
	return !tokenRouteQuotaUntil(tokenId, stepId).IsZero()
}

func tokenRouteCircuitBase(group, modelName string, relayFormat types.RelayFormat) string {
	replacer := strings.NewReplacer(" ", "_", ":", "_", "/", "_")
	return fmt.Sprintf("token-route:%s:%s:%s", replacer.Replace(group), replacer.Replace(modelName), relayFormat)
}

func routeCircuitOpen(group, modelName string, relayFormat types.RelayFormat) bool {
	base := tokenRouteCircuitBase(group, modelName, relayFormat)
	if common.RedisEnabled && common.RDB != nil {
		return common.RDB.Exists(context.Background(), base+":open").Val() > 0
	}
	localRouteCircuit.Lock()
	defer localRouteCircuit.Unlock()
	until := localRouteCircuit.open[base]
	if !until.IsZero() && time.Now().Before(until) {
		return true
	}
	delete(localRouteCircuit.open, base)
	return false
}

func tokenRouteStepAvailability(userId int, userGroup, modelName string, step model.TokenRouteStep, requiredAmount int64) (bool, int64) {
	switch step.FundingSource {
	case model.TokenRouteSourceWallet:
		return step.GroupName == userGroup || GroupInUserUsableGroups(userGroup, step.GroupName), 0
	case model.TokenRouteSourceSubscription, model.TokenRouteSourceVirtualMembership:
		availability, err := model.GetTokenRouteQuotaAvailability(userId, modelName, step, requiredAmount)
		return err == nil && availability.Usable, availability.ResetAt
	default:
		return false, 0
	}
}

// SelectTokenRouteStep resolves the first currently eligible route. Protected
// entitlement slots keep their configured positions; wallet slots use the
// API-key/model-specific runtime order after repeated unavailability.
func SelectTokenRouteStep(c *gin.Context, modelName string, relayFormat types.RelayFormat) (*model.TokenRouteStep, error) {
	if c == nil || common.GetContextKeyString(c, constant.ContextKeyTokenRoutingMode) != model.TokenRoutingModeCustom {
		return nil, nil
	}
	userId := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	tokenId := common.GetContextKeyInt(c, constant.ContextKeyTokenId)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, modelName)
	steps, err := model.GetTokenRouteSteps(userId, tokenId)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, errors.New("API Key 消耗路由策略为空")
	}
	userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
	var unavailable []string
	steps = orderTokenRouteSteps(tokenId, modelName, relayFormat, steps)
	for i := range steps {
		step := steps[i]
		if tokenRouteQuotaCooling(tokenId, step.Id) {
			// Virtual memberships may be reset early by an operator. Probe only
			// their local ledger while frozen; no channel/upstream is visited.
			if step.FundingSource == model.TokenRouteSourceVirtualMembership {
				requiredAmount := tokenRouteQuotaRequiredAmount(tokenId, step.Id)
				usable, resetAt := tokenRouteStepAvailability(userId, userGroup, modelName, step, requiredAmount)
				if usable {
					clearTokenRouteQuotaFreeze(tokenId, step.Id)
				} else {
					if resetAt > 0 {
						freezeTokenRouteQuota(tokenId, step.Id, resetAt, requiredAmount)
					}
					unavailable = append(unavailable, step.GroupName+"（额度冻结中）")
					continue
				}
			} else {
				unavailable = append(unavailable, step.GroupName+"（额度冻结中）")
				continue
			}
		}
		if routeCircuitOpen(step.GroupName, modelName, relayFormat) {
			unavailable = append(unavailable, step.GroupName+"（故障冷却中）")
			continue
		}
		eligible, resetAt := tokenRouteStepAvailability(userId, userGroup, modelName, step, 1)
		if !eligible {
			if step.FundingSource == model.TokenRouteSourceWallet {
				recordWalletRouteUnavailable(tokenId, step.Id, modelName, relayFormat)
			} else {
				freezeTokenRouteQuota(tokenId, step.Id, resetAt, 1)
			}
			unavailable = append(unavailable, step.GroupName+"（额度不可用）")
			continue
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, step.GroupName)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, step.GroupName)
		common.SetContextKey(c, constant.ContextKeyTokenRouteStepId, step.Id)
		common.SetContextKey(c, constant.ContextKeyTokenRoutePosition, i+1)
		common.SetContextKey(c, constant.ContextKeyTokenRouteSource, step.FundingSource)
		common.SetContextKey(c, constant.ContextKeyTokenRouteSelection, step.SelectionMode)
		common.SetContextKey(c, constant.ContextKeyTokenRouteSourceId, step.SourceId)
		c.Set("token_route_configured", true)
		return &step, nil
	}
	return nil, fmt.Errorf("已选择的消耗路线当前均不可用：%s", strings.Join(unavailable, "、"))
}

// MarkCurrentTokenRouteQuotaUnavailable handles the narrow race where a
// ledger looked usable during routing but could not cover the calculated
// pre-consume amount. Entitlement freezes use the ledger reset epoch; wallet
// routes are reordered after repeated unavailability and are never frozen.
func MarkCurrentTokenRouteQuotaUnavailable(c *gin.Context, requiredAmount int64) {
	if c == nil || !c.GetBool("token_route_configured") {
		return
	}
	tokenId := common.GetContextKeyInt(c, constant.ContextKeyTokenId)
	stepId := common.GetContextKeyInt(c, constant.ContextKeyTokenRouteStepId)
	modelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	relayFormat := model.PathToRelayFormat(c.Request.URL.Path)
	step := model.TokenRouteStep{
		Id: stepId, TokenId: tokenId,
		GroupName:     common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
		FundingSource: common.GetContextKeyString(c, constant.ContextKeyTokenRouteSource),
		SelectionMode: common.GetContextKeyString(c, constant.ContextKeyTokenRouteSelection),
		SourceId:      common.GetContextKeyInt(c, constant.ContextKeyTokenRouteSourceId),
	}
	if step.FundingSource == model.TokenRouteSourceWallet {
		recordWalletRouteUnavailable(tokenId, stepId, modelName, relayFormat)
		return
	}
	availability, err := model.GetTokenRouteQuotaAvailability(
		common.GetContextKeyInt(c, constant.ContextKeyUserId), modelName, step, requiredAmount,
	)
	if err != nil || availability.Usable {
		freezeTokenRouteQuota(tokenId, stepId, 0, requiredAmount)
		return
	}
	freezeTokenRouteQuota(tokenId, stepId, availability.ResetAt, requiredAmount)
}

func shouldCountTokenRouteFailure(err *types.NewAPIError) bool {
	if err == nil || err.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
		return false
	}
	if types.IsSkipRetryError(err) || err.StatusCode == 400 || err.StatusCode == 404 || err.StatusCode == 422 {
		return false
	}
	return err.StatusCode == 0 || err.StatusCode == 401 || err.StatusCode == 403 || err.StatusCode == 429 || err.StatusCode >= 500
}

func RecordTokenRouteFailure(c *gin.Context, err *types.NewAPIError) {
	if c == nil || !c.GetBool("token_route_configured") || !shouldCountTokenRouteFailure(err) {
		return
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	modelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	relayFormat := model.PathToRelayFormat(c.Request.URL.Path)
	if common.GetContextKeyString(c, constant.ContextKeyTokenRouteSource) == model.TokenRouteSourceWallet {
		recordWalletRouteUnavailable(
			common.GetContextKeyInt(c, constant.ContextKeyTokenId),
			common.GetContextKeyInt(c, constant.ContextKeyTokenRouteStepId),
			modelName,
			relayFormat,
		)
		return
	}
	base := tokenRouteCircuitBase(group, modelName, relayFormat)
	if common.RedisEnabled && common.RDB != nil {
		ctx := context.Background()
		count, incrErr := common.RDB.Incr(ctx, base+":fail").Result()
		if incrErr != nil {
			return
		}
		if count == 1 {
			common.RDB.Expire(ctx, base+":fail", routeFailureWindow)
		}
		if count >= routeFailureLimit {
			common.RDB.Set(ctx, base+":open", "1", routeOpenDuration)
		}
		return
	}
	now := time.Now()
	localRouteCircuit.Lock()
	window := localRouteCircuit.failures[base][:0]
	for _, at := range localRouteCircuit.failures[base] {
		if now.Sub(at) <= routeFailureWindow {
			window = append(window, at)
		}
	}
	window = append(window, now)
	localRouteCircuit.failures[base] = window
	if len(window) >= routeFailureLimit {
		localRouteCircuit.open[base] = now.Add(routeOpenDuration)
	}
	localRouteCircuit.Unlock()
}

// OpenTokenRouteCircuit immediately cools a route when channel selection has
// already proved that the group cannot serve the requested model/format.
func OpenTokenRouteCircuit(c *gin.Context) {
	if c == nil || !c.GetBool("token_route_configured") {
		return
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	modelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	relayFormat := model.PathToRelayFormat(c.Request.URL.Path)
	if common.GetContextKeyString(c, constant.ContextKeyTokenRouteSource) == model.TokenRouteSourceWallet {
		recordWalletRouteUnavailable(
			common.GetContextKeyInt(c, constant.ContextKeyTokenId),
			common.GetContextKeyInt(c, constant.ContextKeyTokenRouteStepId),
			modelName,
			relayFormat,
		)
		return
	}
	base := tokenRouteCircuitBase(group, modelName, relayFormat)
	if common.RedisEnabled && common.RDB != nil {
		_ = common.RDB.Set(context.Background(), base+":open", "1", routeOpenDuration).Err()
		return
	}
	localRouteCircuit.Lock()
	localRouteCircuit.open[base] = time.Now().Add(routeOpenDuration)
	localRouteCircuit.Unlock()
}

func RecordTokenRouteSuccess(c *gin.Context) {
	if c == nil || !c.GetBool("token_route_configured") || c.GetBool("token_route_terminal_error") {
		return
	}
	group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	modelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	relayFormat := model.PathToRelayFormat(c.Request.URL.Path)
	if common.GetContextKeyString(c, constant.ContextKeyTokenRouteSource) == model.TokenRouteSourceWallet {
		clearWalletRouteFailures(
			common.GetContextKeyInt(c, constant.ContextKeyTokenId),
			common.GetContextKeyInt(c, constant.ContextKeyTokenRouteStepId),
			modelName,
			relayFormat,
		)
	}
	base := tokenRouteCircuitBase(group, modelName, relayFormat)
	quotaKey := tokenRouteQuotaKey(
		common.GetContextKeyInt(c, constant.ContextKeyTokenId),
		common.GetContextKeyInt(c, constant.ContextKeyTokenRouteStepId),
	)
	if common.RedisEnabled && common.RDB != nil {
		_ = common.RDB.Del(context.Background(), base+":fail", base+":open", quotaKey, quotaKey+":amount").Err()
		return
	}
	localRouteCircuit.Lock()
	delete(localRouteCircuit.failures, base)
	delete(localRouteCircuit.open, base)
	delete(localRouteCircuit.quota, quotaKey)
	delete(localRouteCircuit.quotaAmount, quotaKey)
	localRouteCircuit.Unlock()
}

func CurrentTokenRouteSource(c *gin.Context) (source, selection string, sourceId int, ok bool) {
	if c == nil || !c.GetBool("token_route_configured") {
		return "", "", 0, false
	}
	return common.GetContextKeyString(c, constant.ContextKeyTokenRouteSource),
		common.GetContextKeyString(c, constant.ContextKeyTokenRouteSelection),
		common.GetContextKeyInt(c, constant.ContextKeyTokenRouteSourceId), true
}
