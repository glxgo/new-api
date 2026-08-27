package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

func buildMaskedTokenResponse(token *model.Token) *model.Token {
	if token == nil {
		return nil
	}
	maskedToken := *token
	maskedToken.Key = token.GetMaskedKey()
	return &maskedToken
}

func buildMaskedTokenResponses(tokens []*model.Token) []*model.Token {
	maskedTokens := make([]*model.Token, 0, len(tokens))
	for _, token := range tokens {
		maskedTokens = append(maskedTokens, buildMaskedTokenResponse(token))
	}
	return maskedTokens
}

func enrichTokenUsageStats(tokens []*model.Token) {
	if err := model.AttachTokenUsageStats(tokens, time.Now()); err != nil {
		// Usage statistics are informative fields. A temporary log-database
		// failure must not make API-key management unavailable.
		common.SysLog("failed to attach token usage stats: " + err.Error())
	}
}

func decodeTokenMutation(c *gin.Context, token *model.Token) (bool, bool, error) {
	body, err := c.GetRawData()
	if err != nil {
		return false, false, err
	}
	if err := common.Unmarshal(body, token); err != nil {
		return false, false, err
	}
	var fields map[string]any
	if err := common.Unmarshal(body, &fields); err != nil {
		return false, false, err
	}
	_, subscriptionBindingProvided := fields["subscription_mode"]
	_, virtualMembershipProvided := fields["virtual_membership_id"]
	_, virtualMembershipModeProvided := fields["virtual_membership_mode"]
	_, routingModeProvided := fields["routing_mode"]
	_, routeStepsProvided := fields["route_steps"]
	return subscriptionBindingProvided || virtualMembershipProvided || virtualMembershipModeProvided,
		routingModeProvided || routeStepsProvided, nil
}

func tokenBindingInput(token *model.Token) model.TokenSubscriptionBindingInput {
	if token == nil {
		return model.TokenSubscriptionBindingInput{Mode: model.TokenSubscriptionModeAuto}
	}
	return model.TokenSubscriptionBindingInput{
		Mode:           token.SubscriptionMode,
		SubscriptionId: token.SubscriptionId,
		AllowRenewal:   token.SubscriptionAllowRenewal,
		AllowSameGroup: token.SubscriptionAllowSameGroup,
		AllowWallet:    token.SubscriptionAllowWallet,
		WalletLimit:    token.SubscriptionWalletLimit,
		CancelPlanned:  token.CancelPlannedSubscription,
	}
}

func validateTokenRouteWalletGroups(userId int, steps []model.TokenRouteStep) error {
	userGroup, err := model.GetUserGroup(userId, false)
	if err != nil {
		return err
	}
	for _, step := range steps {
		if step.FundingSource == model.TokenRouteSourceWallet &&
			!service.GroupInUserUsableGroups(userGroup, step.GroupName) {
			return fmt.Errorf("无权将分组 %s 加入 API Key 消耗路由策略", step.GroupName)
		}
	}
	return nil
}

func GetAllTokens(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	tokens, err := model.GetAllUserTokens(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enrichTokenUsageStats(tokens)
	total, _ := model.CountUserTokens(userId)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens))
	common.ApiSuccess(c, pageInfo)
}

func SearchTokens(c *gin.Context) {
	userId := c.GetInt("id")
	keyword := c.Query("keyword")
	token := c.Query("token")

	pageInfo := common.GetPageQuery(c)

	tokens, total, err := model.SearchUserTokens(userId, keyword, token, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enrichTokenUsageStats(tokens)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens))
	common.ApiSuccess(c, pageInfo)
}

func GetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.AttachTokenRouteSteps(token); err != nil {
		common.ApiError(c, err)
		return
	}
	enrichTokenUsageStats([]*model.Token{token})
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}

func GetTokenKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"key": token.GetFullKey(),
	})
}

func GetTokenStatus(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	userId := c.GetInt("id")
	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"object":          "credit_summary",
		"total_granted":   token.RemainQuota,
		"total_used":      0, // not supported currently
		"total_available": token.RemainQuota,
		"expires_at":      expiredAt * 1000,
	})
}

func GetTokenUsage(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "No Authorization header",
		})
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Invalid Bearer token",
		})
		return
	}
	tokenKey := parts[1]

	token, err := model.GetTokenByKey(strings.TrimPrefix(tokenKey, "sk-"), false)
	if err != nil {
		common.SysError("failed to get token by key: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgTokenGetInfoFailed)
		return
	}

	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    true,
		"message": "ok",
		"data": gin.H{
			"object":               "token_usage",
			"name":                 token.Name,
			"total_granted":        token.RemainQuota + token.UsedQuota,
			"total_used":           token.UsedQuota,
			"total_available":      token.RemainQuota,
			"unlimited_quota":      token.UnlimitedQuota,
			"model_limits":         token.GetModelLimitsMap(),
			"model_limits_enabled": token.ModelLimitsEnabled,
			"expires_at":           expiredAt,
		},
	})
}

func AddToken(c *gin.Context) {
	token := model.Token{}
	bindingProvided, _, err := decodeTokenMutation(c, &token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	// 非无限额度时，检查额度值是否超出有效范围
	if !token.UnlimitedQuota {
		if token.RemainQuota < 0 {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return
		}
		maxQuotaValue := int((1000000000 * common.QuotaPerUnit))
		if token.RemainQuota > maxQuotaValue {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
			return
		}
	}
	// 检查用户令牌数量是否已达上限
	maxTokens := operation_setting.GetMaxUserTokens()
	count, err := model.CountUserTokens(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if int(count) >= maxTokens {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("已达到最大令牌数量限制 (%d)", maxTokens),
		})
		return
	}
	userId := c.GetInt("id")
	routingMode := model.NormalizeTokenRoutingMode(token.RoutingMode)
	var preparedRoute []model.TokenRouteStep
	bindingInput := tokenBindingInput(&token)
	virtualMembershipId := 0
	virtualMembershipMode := model.NormalizeVirtualMembershipMode(token.VirtualMembershipMode)
	if routingMode == model.TokenRoutingModeCustom {
		preparedRoute, err = model.PrepareTokenRouteSteps(userId, token.RouteSteps)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if err = validateTokenRouteWalletGroups(userId, preparedRoute); err != nil {
			common.ApiError(c, err)
			return
		}
		token.Group = preparedRoute[0].GroupName
		bindingInput = model.TokenSubscriptionBindingInput{Mode: model.TokenSubscriptionModeAuto, CancelPlanned: true}
		virtualMembershipMode = model.VirtualMembershipModeInstance
	} else {
		if strings.TrimSpace(token.Group) == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "请选择令牌分组",
			})
			return
		}
		if !bindingProvided {
			bindingInput.Mode = model.TokenSubscriptionModeAuto
		}
		bindingInput, virtualMembershipId, virtualMembershipMode, err = model.ResolveTokenFundingBindingForGroupWithMode(
			userId,
			strings.TrimSpace(token.Group),
			bindingInput,
			token.VirtualMembershipId,
			virtualMembershipMode,
		)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	key, err := common.GenerateKey()
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgTokenGenerateFailed)
		common.SysLog("failed to generate token key: " + err.Error())
		return
	}
	cleanToken := model.Token{
		UserId:                userId,
		Name:                  token.Name,
		Key:                   key,
		CreatedTime:           common.GetTimestamp(),
		AccessedTime:          common.GetTimestamp(),
		ExpiredTime:           token.ExpiredTime,
		RemainQuota:           token.RemainQuota,
		UnlimitedQuota:        token.UnlimitedQuota,
		ModelLimitsEnabled:    token.ModelLimitsEnabled,
		ModelLimits:           token.ModelLimits,
		AllowIps:              token.AllowIps,
		Group:                 strings.TrimSpace(token.Group),
		CrossGroupRetry:       token.CrossGroupRetry,
		RoutingMode:           routingMode,
		VirtualMembershipMode: virtualMembershipMode,
	}
	model.ApplyTokenSubscriptionBindingInput(&cleanToken, bindingInput)
	cleanToken.VirtualMembershipId = virtualMembershipId
	if routingMode == model.TokenRoutingModeCustom {
		err = model.InsertTokenWithRoute(&cleanToken, preparedRoute)
	} else {
		err = cleanToken.Insert()
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if routingMode != model.TokenRoutingModeCustom {
		if err := model.RecordInitialTokenSubscriptionBinding(&cleanToken, "API key created"); err != nil {
			common.SysLog("failed to record initial token subscription binding: " + err.Error())
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	err := model.DeleteTokenById(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateToken(c *gin.Context) {
	userId := c.GetInt("id")
	statusOnly := c.Query("status_only")
	token := model.Token{}
	bindingProvided, routingProvided, err := decodeTokenMutation(c, &token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	if !token.UnlimitedQuota {
		if token.RemainQuota < 0 {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return
		}
		maxQuotaValue := int((1000000000 * common.QuotaPerUnit))
		if token.RemainQuota > maxQuotaValue {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
			return
		}
	}
	cleanToken, err := model.GetTokenByIds(token.Id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if token.Status == common.TokenStatusEnabled {
		if cleanToken.Status == common.TokenStatusExpired && cleanToken.ExpiredTime <= common.GetTimestamp() && cleanToken.ExpiredTime != -1 {
			common.ApiErrorI18n(c, i18n.MsgTokenExpiredCannotEnable)
			return
		}
		if cleanToken.Status == common.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota {
			common.ApiErrorI18n(c, i18n.MsgTokenExhaustedCannotEable)
			return
		}
	}
	if statusOnly != "" {
		cleanToken.Status = token.Status
	} else {
		routingMode := model.NormalizeTokenRoutingMode(cleanToken.RoutingMode)
		if routingProvided {
			routingMode = model.NormalizeTokenRoutingMode(token.RoutingMode)
		}
		if routingMode == model.TokenRoutingModeCustom {
			preparedRoute, prepareErr := model.PrepareTokenRouteSteps(userId, token.RouteSteps)
			if prepareErr != nil {
				common.ApiError(c, prepareErr)
				return
			}
			if prepareErr = validateTokenRouteWalletGroups(userId, preparedRoute); prepareErr != nil {
				common.ApiError(c, prepareErr)
				return
			}
			token.Group = preparedRoute[0].GroupName
			cleanToken.Name = token.Name
			cleanToken.ExpiredTime = token.ExpiredTime
			cleanToken.RemainQuota = token.RemainQuota
			cleanToken.UnlimitedQuota = token.UnlimitedQuota
			cleanToken.ModelLimitsEnabled = token.ModelLimitsEnabled
			cleanToken.ModelLimits = token.ModelLimits
			cleanToken.AllowIps = token.AllowIps
			cleanToken.Group = token.Group
			cleanToken.RoutingMode = model.TokenRoutingModeCustom
			updated, routeErr := model.UpdateTokenWithRoute(userId, cleanToken, preparedRoute, token.RoutingRevision)
			if routeErr != nil {
				common.ApiError(c, routeErr)
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "",
				"data":    buildMaskedTokenResponse(updated),
			})
			return
		}
		if strings.TrimSpace(token.Group) == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "请选择令牌分组",
			})
			return
		}
		bindingInput := tokenBindingInput(&token)
		if model.NormalizeTokenRoutingMode(cleanToken.RoutingMode) == model.TokenRoutingModeCustom {
			bindingProvided = true
			bindingInput.Mode = model.TokenSubscriptionModeAuto
		}
		if bindingProvided {
			bindingInput, virtualMembershipId, virtualMembershipMode, resolveErr := model.ResolveTokenFundingBindingForGroupWithMode(
				userId,
				strings.TrimSpace(token.Group),
				bindingInput,
				token.VirtualMembershipId,
				token.VirtualMembershipMode,
			)
			if resolveErr != nil {
				common.ApiError(c, resolveErr)
				return
			}
			model.ApplyTokenSubscriptionBindingInput(&token, bindingInput)
			token.VirtualMembershipId = virtualMembershipId
			token.VirtualMembershipMode = virtualMembershipMode
		} else if model.NormalizeTokenSubscriptionMode(cleanToken.SubscriptionMode) == model.TokenSubscriptionModeInstance &&
			cleanToken.SubscriptionId > 0 {
			if _, err := model.ValidateSubscriptionForToken(
				userId,
				strings.TrimSpace(token.Group),
				cleanToken.SubscriptionId,
				false,
			); err != nil {
				common.ApiError(c, err)
				return
			}
		}
		// If you add more fields, please also update token.Update()
		cleanToken.Name = token.Name
		cleanToken.ExpiredTime = token.ExpiredTime
		cleanToken.RemainQuota = token.RemainQuota
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		cleanToken.ModelLimitsEnabled = token.ModelLimitsEnabled
		cleanToken.ModelLimits = token.ModelLimits
		cleanToken.AllowIps = token.AllowIps
		cleanToken.Group = strings.TrimSpace(token.Group)
		cleanToken.CrossGroupRetry = token.CrossGroupRetry
		cleanToken.RoutingMode = model.TokenRoutingModeSingle
		cleanToken.RoutingRevision = token.RoutingRevision
		if bindingProvided {
			cleanToken.VirtualMembershipId = token.VirtualMembershipId
			cleanToken.VirtualMembershipMode = token.VirtualMembershipMode
		}
	}
	if statusOnly == "" && bindingProvided {
		updated, bindingErr := model.UpdateTokenWithSubscriptionBinding(
			userId,
			cleanToken,
			tokenBindingInput(&token),
			"user",
		)
		if bindingErr != nil {
			common.ApiError(c, bindingErr)
			return
		}
		cleanToken = updated
	} else {
		err = cleanToken.Update()
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildMaskedTokenResponse(cleanToken),
	})
}

type TokenBatch struct {
	Ids []int `json:"ids"`
}

func DeleteTokenBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	count, err := model.BatchDeleteTokens(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}

func GetTokenKeysBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if len(tokenBatch.Ids) > 100 {
		common.ApiErrorI18n(c, i18n.MsgBatchTooMany, map[string]any{"Max": 100})
		return
	}
	userId := c.GetInt("id")
	tokens, err := model.GetTokenKeysByIds(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	keysMap := make(map[int]string)
	for _, t := range tokens {
		keysMap[t.Id] = t.GetFullKey()
	}
	common.ApiSuccess(c, gin.H{"keys": keysMap})
}
