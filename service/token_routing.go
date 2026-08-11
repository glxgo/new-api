package service

import (
	"context"
	"errors"
	"fmt"
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
	failures map[string][]time.Time
	open     map[string]time.Time
	quota    map[string]time.Time
}{failures: map[string][]time.Time{}, open: map[string]time.Time{}, quota: map[string]time.Time{}}

func tokenRouteQuotaKey(tokenId, stepId int) string {
	return fmt.Sprintf("token-route-quota:%d:%d", tokenId, stepId)
}

func tokenRouteQuotaCooling(tokenId, stepId int) bool {
	key := tokenRouteQuotaKey(tokenId, stepId)
	if common.RedisEnabled && common.RDB != nil {
		return common.RDB.Exists(context.Background(), key).Val() > 0
	}
	localRouteCircuit.Lock()
	defer localRouteCircuit.Unlock()
	until := localRouteCircuit.quota[key]
	if !until.IsZero() && time.Now().Before(until) {
		return true
	}
	delete(localRouteCircuit.quota, key)
	return false
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

func tokenRouteStepEligible(userId int, userGroup, modelName string, step model.TokenRouteStep) bool {
	switch step.FundingSource {
	case model.TokenRouteSourceWallet:
		return step.GroupName == userGroup || GroupInUserUsableGroups(userGroup, step.GroupName)
	case model.TokenRouteSourceSubscription:
		if step.SelectionMode == model.TokenRouteSelectionInstance {
			_, err := model.ValidateSubscriptionForToken(userId, step.GroupName, step.SourceId, true)
			return err == nil
		}
		has, err := model.HasUsableUserSubscriptionByGroup(userId, step.GroupName)
		return err == nil && has
	case model.TokenRouteSourceVirtualMembership:
		if step.SelectionMode == model.TokenRouteSelectionInstance {
			if err := model.ValidateVirtualMembershipForToken(userId, step.GroupName, step.SourceId, true); err != nil {
				return false
			}
			membership, err := model.GetVirtualMembershipByIdForUser(userId, step.SourceId)
			if err != nil || membership == nil || strings.TrimSpace(membership.AllowedModels) == "" {
				return err == nil
			}
			for _, allowed := range strings.Split(membership.AllowedModels, ",") {
				if strings.TrimSpace(allowed) == modelName {
					return true
				}
			}
			return false
		}
		has, err := model.HasUsableVirtualMembershipForRoute(userId, step.GroupName, modelName)
		return err == nil && has
	default:
		return false
	}
}

// SelectTokenRouteStep resolves the first currently eligible route. Every
// request starts from position one, so restored quota automatically regains
// priority. Open circuits are skipped until their short TTL expires.
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
	for i := range steps {
		step := steps[i]
		if tokenRouteQuotaCooling(tokenId, step.Id) {
			unavailable = append(unavailable, step.GroupName+"（额度冷却中）")
			continue
		}
		if routeCircuitOpen(step.GroupName, modelName, relayFormat) {
			unavailable = append(unavailable, step.GroupName+"（故障冷却中）")
			continue
		}
		if !tokenRouteStepEligible(userId, userGroup, modelName, step) {
			unavailable = append(unavailable, step.GroupName+"（额度不可用）")
			continue
		}
		common.SetContextKey(c, constant.ContextKeyUsingGroup, step.GroupName)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, step.GroupName)
		common.SetContextKey(c, constant.ContextKeyTokenRouteStepId, step.Id)
		common.SetContextKey(c, constant.ContextKeyTokenRoutePosition, step.Position)
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
// pre-consume amount. The cooldown is token/step scoped, never global.
func MarkCurrentTokenRouteQuotaUnavailable(c *gin.Context) {
	if c == nil || !c.GetBool("token_route_configured") {
		return
	}
	key := tokenRouteQuotaKey(
		common.GetContextKeyInt(c, constant.ContextKeyTokenId),
		common.GetContextKeyInt(c, constant.ContextKeyTokenRouteStepId),
	)
	if common.RedisEnabled && common.RDB != nil {
		_ = common.RDB.Set(context.Background(), key, "1", routeOpenDuration).Err()
		return
	}
	localRouteCircuit.Lock()
	localRouteCircuit.quota[key] = time.Now().Add(routeOpenDuration)
	localRouteCircuit.Unlock()
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
	base := tokenRouteCircuitBase(group, modelName, relayFormat)
	quotaKey := tokenRouteQuotaKey(
		common.GetContextKeyInt(c, constant.ContextKeyTokenId),
		common.GetContextKeyInt(c, constant.ContextKeyTokenRouteStepId),
	)
	if common.RedisEnabled && common.RDB != nil {
		_ = common.RDB.Del(context.Background(), base+":fail", base+":open", quotaKey).Err()
		return
	}
	localRouteCircuit.Lock()
	delete(localRouteCircuit.failures, base)
	delete(localRouteCircuit.open, base)
	delete(localRouteCircuit.quota, quotaKey)
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
