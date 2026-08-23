package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	// Import oauth package to register providers via init()
	_ "github.com/QuantumNous/new-api/oauth"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	anonymousRequestBodyLimit := middleware.AnonymousRequestBodyLimit()
	{
		apiRouter.GET("/setup", controller.GetSetup)
		apiRouter.POST("/setup", anonymousRequestBodyLimit, controller.PostSetup)
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/uptime/status", controller.GetUptimeKumaStatus)
		apiRouter.GET("/models", middleware.UserAuth(), controller.DashboardListModels)
		apiRouter.GET("/status/test", middleware.AdminAuth(), controller.TestStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/user-agreement", controller.GetUserAgreement)
		apiRouter.GET("/privacy-policy", controller.GetPrivacyPolicy)
		apiRouter.GET("/about", controller.GetAbout)
		apiRouter.GET("/tutorial", controller.GetTutorial)
		ingressPingRoute := apiRouter.Group("/ingress")
		ingressPingRoute.Use(middleware.PublicProbeCORS())
		ingressPingRoute.GET("/ping", middleware.APIIngressResolver(), controller.APIIngressPing)
		ingressPingRoute.OPTIONS("/ping")
		//apiRouter.GET("/midjourney", controller.GetMidjourney)
		apiRouter.GET("/home_page_content", controller.GetHomePageContent)
		apiRouter.GET("/pricing", middleware.HeaderNavModuleAuth("pricing"), controller.GetPricing)
		perfMetricsRoute := apiRouter.Group("/perf-metrics")
		// no-store: 防止 CF/CDN 缓存动态 perf-metrics 接口(否则新接口上线前会被缓存 404)
		perfMetricsRoute.Use(func(c *gin.Context) {
			c.Header("Cache-Control", "no-store")
			c.Next()
		})
		perfMetricsRoute.Use(middleware.HeaderNavModulePublicOrUserAuth("pricing"))
		{
			perfMetricsRoute.GET("/summary", controller.GetPerfMetricsSummary)
			perfMetricsRoute.GET("/group-summary", controller.GetPerfMetricsGroupSummary)
			perfMetricsRoute.GET("", controller.GetPerfMetrics)
		}
		apiRouter.GET("/rankings", middleware.HeaderNavModuleAuth("rankings"), controller.GetRankings)
		apiRouter.GET("/verification", middleware.EmailVerificationRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.ResetPassword)
		// OAuth routes - specific routes must come before :provider wildcard
		apiRouter.GET("/oauth/state", middleware.CriticalRateLimit(), controller.GenerateOAuthCode)
		apiRouter.POST("/oauth/email/bind", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.EmailBind)
		// Non-standard OAuth (WeChat, Telegram) - keep original routes
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), controller.WeChatAuth)
		apiRouter.POST("/oauth/wechat/bind", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.WeChatBind)
		apiRouter.GET("/oauth/telegram/login", middleware.CriticalRateLimit(), controller.TelegramLogin)
		apiRouter.GET("/oauth/telegram/bind", middleware.CriticalRateLimit(), controller.TelegramBind)
		// Standard OAuth providers (GitHub, Discord, OIDC, LinuxDO) - unified route
		apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), controller.HandleOAuth)
		apiRouter.GET("/ratio_config", middleware.CriticalRateLimit(), controller.GetRatioConfig)

		apiRouter.POST("/stripe/webhook", anonymousRequestBodyLimit, controller.StripeWebhook)
		apiRouter.POST("/creem/webhook", anonymousRequestBodyLimit, controller.CreemWebhook)
		apiRouter.POST("/waffo/webhook", anonymousRequestBodyLimit, controller.WaffoWebhook)
		// :env separates test vs prod URLs so the operator can register each
		// in Pancake's matching webhook slot; handler enforces env match.
		apiRouter.POST("/waffo-pancake/webhook/:env", anonymousRequestBodyLimit, controller.WaffoPancakeWebhook)

		// Universal secure verification routes
		apiRouter.POST("/verify", middleware.UserAuth(), middleware.CriticalRateLimit(), controller.UniversalVerify)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Login)
			userRoute.POST("/login/2fa", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.Verify2FALogin)
			userRoute.POST("/passkey/login/begin", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.PasskeyLoginBegin)
			userRoute.POST("/passkey/login/finish", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.PasskeyLoginFinish)
			//userRoute.POST("/tokenlog", middleware.CriticalRateLimit(), controller.TokenLog)
			userRoute.GET("/logout", middleware.SessionCookieOriginGuard(), controller.Logout)
			userRoute.POST("/epay/notify", anonymousRequestBodyLimit, controller.EpayNotify)
			userRoute.GET("/epay/notify", controller.EpayNotify)
			userRoute.GET("/groups", controller.GetUserGroups)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth())
			{
				selfRoute.GET("/self/groups", controller.GetUserGroups)
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.GET("/self/identity-access", controller.GetSelfIdentityAccess)
				selfRoute.POST("/self/mainland-whitelist", middleware.CriticalRateLimit(), controller.ApplyMainlandWhitelist)
				selfRoute.GET("/models", controller.GetUserModels)
				selfRoute.GET("/cache-rate", controller.GetUserCacheRate)
				selfRoute.GET("/concurrency-applications", controller.GetSelfConcurrencyApplications)
				selfRoute.POST("/concurrency-applications", middleware.CriticalRateLimit(), controller.CreateConcurrencyApplication)
				selfRoute.PUT("/self", controller.UpdateSelf)
				selfRoute.DELETE("/self", controller.DeleteSelf)
				selfRoute.GET("/token", controller.GenerateAccessToken)
				selfRoute.GET("/passkey", controller.PasskeyStatus)
				selfRoute.POST("/passkey/register/begin", controller.PasskeyRegisterBegin)
				selfRoute.POST("/passkey/register/finish", controller.PasskeyRegisterFinish)
				selfRoute.POST("/passkey/verify/begin", controller.PasskeyVerifyBegin)
				selfRoute.POST("/passkey/verify/finish", controller.PasskeyVerifyFinish)
				selfRoute.DELETE("/passkey", controller.PasskeyDelete)
				selfRoute.GET("/aff", controller.GetAffCode)
				selfRoute.GET("/topup/info", controller.GetTopUpInfo)
				selfRoute.GET("/topup/self", controller.GetUserTopUps)
				selfRoute.POST("/topup", middleware.CriticalRateLimit(), controller.TopUp)
				selfRoute.POST("/pay", middleware.CriticalRateLimit(), controller.RequestEpay)
				selfRoute.POST("/amount", controller.RequestAmount)
				selfRoute.POST("/topup/coupon/preview", controller.PreviewTopUpCoupon)
				selfRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.RequestStripePay)
				selfRoute.POST("/stripe/amount", controller.RequestStripeAmount)
				selfRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.RequestCreemPay)
				selfRoute.POST("/waffo/amount", controller.RequestWaffoAmount)
				selfRoute.POST("/waffo/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPay)
				selfRoute.POST("/waffo-pancake/amount", controller.RequestWaffoPancakeAmount)
				selfRoute.POST("/waffo-pancake/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPancakePay)
				selfRoute.POST("/aff_transfer", controller.TransferAffQuota)
				selfRoute.PUT("/setting", controller.UpdateUserSetting)
				selfRoute.GET("/announcements", controller.GetUserAnnouncements)
				selfRoute.POST("/announcements/read", controller.MarkUserAnnouncementsRead)

				// 2FA routes
				selfRoute.GET("/2fa/status", controller.Get2FAStatus)
				selfRoute.POST("/2fa/setup", controller.Setup2FA)
				selfRoute.POST("/2fa/enable", controller.Enable2FA)
				selfRoute.POST("/2fa/disable", controller.Disable2FA)
				selfRoute.POST("/2fa/backup_codes", controller.RegenerateBackupCodes)

				// Check-in routes
				selfRoute.GET("/checkin", controller.GetCheckinStatus)
				selfRoute.POST("/checkin", middleware.TurnstileCheck(), controller.DoCheckin)

				// Custom OAuth bindings
				selfRoute.GET("/oauth/bindings", controller.GetUserOAuthBindings)
				selfRoute.DELETE("/oauth/bindings/:provider_id", controller.UnbindCustomOAuth)

				// Withdraw (rebate/dividend system): user apply + list own
				selfRoute.POST("/withdraw", middleware.CriticalRateLimit(), controller.RequestWithdraw)
				selfRoute.GET("/withdraw/self", controller.GetUserWithdraws)
				// Affiliate (邀新计划): summary + downline + rebates
				selfRoute.GET("/affiliate/summary", controller.GetAffiliateSummary)
				selfRoute.GET("/affiliate/downline", controller.GetAffiliateDownline)
				selfRoute.GET("/affiliate/rebates", controller.GetAffiliateRebates)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/", controller.GetAllUsers)
				adminRoute.GET("/topup", controller.GetAllTopUps)
				adminRoute.POST("/topup/complete", controller.AdminCompleteTopUp)
				adminRoute.GET("/search", controller.SearchUsers)
				adminRoute.GET("/:id/oauth/bindings", controller.GetUserOAuthBindingsByAdmin)
				adminRoute.DELETE("/:id/oauth/bindings/:provider_id", controller.UnbindCustomOAuthByAdmin)
				adminRoute.DELETE("/:id/bindings/:binding_type", controller.AdminClearUserBinding)
				adminRoute.PUT("/:id/identity", controller.UpdateUserIdentity)
				adminRoute.GET("/:id", controller.GetUser)
				adminRoute.POST("/", controller.CreateUser)
				adminRoute.POST("/manage", controller.ManageUser)
				adminRoute.PUT("/", controller.UpdateUser)
				adminRoute.DELETE("/:id", controller.DeleteUser)
				adminRoute.DELETE("/:id/reset_passkey", controller.AdminResetPasskey)
				adminRoute.GET("/admin/concurrency-applications", controller.GetConcurrencyApplications)
				adminRoute.POST("/admin/concurrency-applications/:id/review", controller.ReviewConcurrencyApplication)

				// Admin 2FA routes
				adminRoute.GET("/2fa/stats", controller.Admin2FAStats)
				adminRoute.DELETE("/:id/2fa", controller.AdminDisable2FA)
			}
		}

		// Subscription billing (plans, purchase, admin management)
		subscriptionRoute := apiRouter.Group("/subscription")
		subscriptionRoute.Use(middleware.UserAuth())
		{
			subscriptionRoute.GET("/plans", controller.GetSubscriptionPlans)
			subscriptionRoute.GET("/intro", controller.GetSubscriptionIntro)
			subscriptionRoute.GET("/self", controller.GetSubscriptionSelf)
			subscriptionRoute.PUT("/self/preference", controller.UpdateSubscriptionPreference)
			subscriptionRoute.PATCH("/self/instances/:id/visibility", controller.SetSelfSubscriptionVisibility)
			subscriptionRoute.PATCH("/self/instances/:id/remark", controller.UpdateSelfSubscriptionRemark)
			subscriptionRoute.GET("/self/instances/:id/renewal-preview", controller.GetSelfSubscriptionRenewalPreview)
			subscriptionRoute.GET("/self/instances/:id/keys", controller.ListSelfSubscriptionTokenBindings)
			subscriptionRoute.PUT("/self/instances/:id/keys", controller.ReplaceSelfSubscriptionTokenBindings)
			subscriptionRoute.GET("/self/consumption-order", controller.GetSubscriptionConsumptionOrder)
			subscriptionRoute.PUT("/self/consumption-order", controller.UpdateSubscriptionConsumptionOrder)
			subscriptionRoute.POST("/balance/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestBalancePay)
			subscriptionRoute.POST("/epay/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestEpay)
			subscriptionRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestStripePay)
			subscriptionRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestCreemPay)
			subscriptionRoute.POST("/waffo-pancake/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestWaffoPancakePay)
		}

		virtualMembershipRoute := apiRouter.Group("/virtual-membership")
		virtualMembershipRoute.Use(middleware.UserAuth())
		{
			virtualMembershipRoute.GET("/page", controller.GetVirtualMembershipPage)
			virtualMembershipRoute.POST("/balance/pay", middleware.CriticalRateLimit(), controller.PurchaseVirtualMembership)
			virtualMembershipRoute.POST("/epay/pay", middleware.CriticalRateLimit(), controller.VirtualMembershipRequestEpay)
			virtualMembershipRoute.POST("/:id/reset/epay", middleware.CriticalRateLimit(), controller.VirtualMembershipActiveResetRequestEpay)
			virtualMembershipRoute.GET("/:id/keys", controller.ListVirtualMembershipTokens)
			virtualMembershipRoute.PUT("/:id/keys", controller.ReplaceVirtualMembershipTokens)
			virtualMembershipRoute.PATCH("/:id/visibility", controller.SetSelfVirtualMembershipVisibility)
			virtualMembershipRoute.POST("/:id/reset", middleware.CriticalRateLimit(), controller.ActiveResetVirtualMembership)
		}
		virtualMembershipAdminRoute := apiRouter.Group("/virtual-membership/admin")
		virtualMembershipAdminRoute.Use(middleware.AdminAuth())
		{
			virtualMembershipAdminRoute.GET("/plans", controller.AdminListVirtualMembershipPlans)
			virtualMembershipAdminRoute.POST("/plans", controller.AdminSaveVirtualMembershipPlan)
			virtualMembershipAdminRoute.PUT("/plans/:id", controller.AdminSaveVirtualMembershipPlan)
			virtualMembershipAdminRoute.GET("/setting", controller.AdminGetVirtualMembershipSetting)
			virtualMembershipAdminRoute.PUT("/setting", controller.AdminSaveVirtualMembershipSetting)
			virtualMembershipAdminRoute.POST("/reset", controller.AdminResetVirtualMemberships)
			virtualMembershipAdminRoute.GET("/memberships", controller.AdminListVirtualMemberships)
			virtualMembershipAdminRoute.POST("/memberships", controller.AdminGrantVirtualMembership)
			virtualMembershipAdminRoute.POST("/memberships/:id/reset-credits", controller.AdminGrantVirtualMembershipResetCredits)
			virtualMembershipAdminRoute.POST("/memberships/:id/renew", controller.AdminRenewVirtualMembership)
			virtualMembershipAdminRoute.PATCH("/memberships/:id/visibility", controller.AdminSetVirtualMembershipVisibility)
			virtualMembershipAdminRoute.DELETE("/memberships/:id", controller.AdminDeleteVirtualMembership)
			virtualMembershipAdminRoute.GET("/orders", controller.AdminListVirtualMembershipOrders)
		}
		topUpCouponAdminRoute := apiRouter.Group("/topup-coupon/admin")
		topUpCouponAdminRoute.Use(middleware.AdminAuth())
		{
			topUpCouponAdminRoute.GET("", controller.AdminListTopUpCoupons)
			topUpCouponAdminRoute.POST("", controller.AdminSaveTopUpCoupon)
			topUpCouponAdminRoute.PUT("/:id", controller.AdminSaveTopUpCoupon)
			topUpCouponAdminRoute.DELETE("/:id", controller.AdminDeleteTopUpCoupon)
		}

		ingressRoute := apiRouter.Group("/ingress")
		ingressRoute.Use(middleware.UserAuth())
		{
			ingressRoute.GET("/profiles", controller.GetAPIIngressProfiles)
		}
		ingressAdminRoute := apiRouter.Group("/ingress/admin")
		ingressAdminRoute.Use(middleware.AdminAuth())
		{
			ingressAdminRoute.GET("/profiles", controller.AdminListAPIIngressProfiles)
			ingressAdminRoute.POST("/profiles", controller.AdminCreateAPIIngressProfile)
			ingressAdminRoute.PUT("/profiles/:id", controller.AdminUpdateAPIIngressProfile)
			ingressAdminRoute.DELETE("/profiles/:id", controller.AdminDeleteAPIIngressProfile)
		}

		luckyWheelRoute := apiRouter.Group("/lucky-wheel")
		luckyWheelRoute.Use(middleware.UserAuth())
		{
			luckyWheelRoute.GET("/status", controller.GetLuckyWheelStatus)
			luckyWheelRoute.GET("/rules", controller.GetLuckyWheelRules)
			luckyWheelRoute.GET("/cards", controller.GetMyLuckyCards)
			luckyWheelRoute.POST("/draws", middleware.CriticalRateLimit(), controller.CreateLuckyDraw)
			luckyWheelRoute.GET("/draws", controller.GetMyLuckyDraws)
			luckyWheelRoute.GET("/draws/:id", controller.GetLuckyDraw)
		}
		luckyWheelAdminRoute := apiRouter.Group("/lucky-wheel/admin")
		luckyWheelAdminRoute.Use(middleware.AdminAuth())
		{
			luckyWheelAdminRoute.GET("/overview", controller.AdminLuckyOverview)
			luckyWheelAdminRoute.GET("/cards", controller.AdminListLuckyCards)
			luckyWheelAdminRoute.GET("/draws", controller.AdminListLuckyDraws)
			luckyWheelAdminRoute.GET("/rule-sets", controller.AdminListLuckyRuleSets)
		}
		luckyWheelRootRoute := apiRouter.Group("/lucky-wheel/admin")
		luckyWheelRootRoute.Use(middleware.RootAuth())
		{
			luckyWheelRootRoute.POST("/cards/compensate", controller.AdminCompensateLuckyCards)
			luckyWheelRootRoute.POST("/cards/revoke-user", controller.AdminRevokeUserLuckyCards)
			luckyWheelRootRoute.POST("/pause-issuance", controller.AdminPauseLuckyIssuance)
			luckyWheelRootRoute.POST("/resume-issuance", controller.AdminResumeLuckyIssuance)
			luckyWheelRootRoute.POST("/pause-draw", controller.AdminPauseLuckyDraw)
			luckyWheelRootRoute.POST("/resume-draw", controller.AdminResumeLuckyDraw)
			luckyWheelRootRoute.POST("/source-reversals", controller.AdminReverseLuckySource)
			luckyWheelRootRoute.POST("/rule-sets", controller.AdminCreateLuckyRuleSet)
			luckyWheelRootRoute.POST("/rule-sets/:id/activate", controller.AdminActivateLuckyRuleSet)
		}
		subscriptionAdminRoute := apiRouter.Group("/subscription/admin")
		subscriptionAdminRoute.Use(middleware.AdminAuth())
		{
			subscriptionAdminRoute.GET("/plans", controller.AdminListSubscriptionPlans)
			subscriptionAdminRoute.GET("/subscribers", controller.GetSubscriptionSubscribers)
			subscriptionAdminRoute.POST("/plans", controller.AdminCreateSubscriptionPlan)
			subscriptionAdminRoute.PUT("/plans/:id", controller.AdminUpdateSubscriptionPlan)
			subscriptionAdminRoute.PATCH("/plans/:id", controller.AdminUpdateSubscriptionPlanStatus)
			subscriptionAdminRoute.POST("/bind", controller.AdminBindSubscription)

			// User subscription management (admin)
			subscriptionAdminRoute.GET("/users/:id/subscriptions", controller.AdminListUserSubscriptions)
			subscriptionAdminRoute.POST("/users/:id/subscriptions", controller.AdminCreateUserSubscription)
			subscriptionAdminRoute.POST("/user_subscriptions/:id/renew", controller.AdminRenewUserSubscription)
			subscriptionAdminRoute.PATCH("/user_subscriptions/:id/visibility", controller.AdminSetUserSubscriptionVisibility)
			subscriptionAdminRoute.POST("/user_subscriptions/:id/invalidate", controller.AdminInvalidateUserSubscription)
			subscriptionAdminRoute.DELETE("/user_subscriptions/:id", controller.AdminDeleteUserSubscription)
		}

		// Subscription payment callbacks (no auth)
		apiRouter.POST("/subscription/epay/notify", anonymousRequestBodyLimit, controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/notify", controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/return", controller.SubscriptionEpayReturn)
		apiRouter.POST("/subscription/epay/return", anonymousRequestBodyLimit, controller.SubscriptionEpayReturn)
		apiRouter.POST("/virtual-membership/epay/notify", anonymousRequestBodyLimit, controller.VirtualMembershipEpayNotify)
		apiRouter.GET("/virtual-membership/epay/notify", controller.VirtualMembershipEpayNotify)
		apiRouter.GET("/virtual-membership/epay/return", controller.VirtualMembershipEpayReturn)
		apiRouter.POST("/virtual-membership/epay/return", anonymousRequestBodyLimit, controller.VirtualMembershipEpayReturn)
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.PUT("/", controller.UpdateOption)
			optionRoute.POST("/upload", controller.UploadImage) // 图片上传(教程配图), RootAuth
			optionRoute.POST("/payment_compliance", controller.ConfirmPaymentCompliance)
			optionRoute.GET("/channel_affinity_cache", controller.GetChannelAffinityCacheStats)
			optionRoute.DELETE("/channel_affinity_cache", controller.ClearChannelAffinityCache)
			optionRoute.POST("/rest_model_ratio", controller.ResetModelRatio)
			optionRoute.POST("/migrate_console_setting", controller.MigrateConsoleSetting) // 用于迁移检测的旧键，下个版本会删除
			optionRoute.POST("/waffo-pancake/catalog", controller.ListWaffoPancakeCatalog)
			optionRoute.POST("/waffo-pancake/pair", controller.CreateWaffoPancakePair)
			optionRoute.POST("/waffo-pancake/save", controller.SaveWaffoPancake)
			optionRoute.POST("/waffo-pancake/subscription-product", controller.CreateWaffoPancakeSubscriptionProduct)
			optionRoute.POST("/waffo-pancake/subscription-product-options", controller.ListWaffoPancakeSubscriptionProductOptions)
		}

		// 中国大陆网页访问限制策略（PRD v0.4）：AdminAuth 可读可改，RootAuth 由 AdminAuth 覆盖。
		accessPolicyRoute := apiRouter.Group("/access-policy")
		accessPolicyRoute.Use(middleware.AdminAuth())
		{
			accessPolicyRoute.GET("", controller.GetAccessPolicy)
			accessPolicyRoute.PUT("", controller.UpdateAccessPolicy)
			accessPolicyRoute.POST("/rollback", controller.RollbackAccessPolicy)
			accessPolicyRoute.GET("/allowlists", controller.ListMainlandAllowlists)
			accessPolicyRoute.DELETE("/allowlists/:id", controller.RevokeMainlandAllowlist)
		}
		// The CTA on the self-contained 451 page uses a same-origin session
		// endpoint. GET is a static form; POST still requires a live session and
		// trusted server-side IP resolution (username is only a confirmation).
		apiRouter.GET("/access-policy/whitelist", controller.GetMainlandWhitelistPage)
		apiRouter.POST("/access-policy/whitelist", middleware.CriticalRateLimit(), middleware.SessionCookieOriginGuard(), controller.ApplyMainlandWhitelistFromSession)

		// 分组定价预览(2026-06-22): 管理员以上可访问, 成本/毛利字段仅 Root 返回(controller 内裁剪)。
		apiRouter.GET("/option/pricing/group-preview", middleware.AdminAuth(), controller.GetGroupPricingPreview)

		// Withdraw review (root only): rebate/dividend withdrawal approval queue
		withdrawRoute := apiRouter.Group("/withdraw")
		withdrawRoute.Use(middleware.RootAuth())
		{
			withdrawRoute.GET("/", controller.GetAllWithdraws)
			withdrawRoute.POST("/approve", controller.ApproveWithdraw)
			withdrawRoute.POST("/reject", controller.RejectWithdraw)
		}

		// Profit dashboard + dividend audit (root only)
		profitRoute := apiRouter.Group("/profit")
		profitRoute.Use(middleware.RootAuth())
		{
			profitRoute.GET("/settings", controller.GetCommissionSettings)
			profitRoute.PUT("/settings", controller.UpdateCommissionSettings)
			profitRoute.GET("/summary", controller.GetProfitSummary)
			profitRoute.GET("/dividend_records", controller.GetDividendRecords)
		}

		// Custom OAuth provider management (root only)
		customOAuthRoute := apiRouter.Group("/custom-oauth-provider")
		customOAuthRoute.Use(middleware.RootAuth())
		{
			customOAuthRoute.POST("/discovery", controller.FetchCustomOAuthDiscovery)
			customOAuthRoute.GET("/", controller.GetCustomOAuthProviders)
			customOAuthRoute.GET("/:id", controller.GetCustomOAuthProvider)
			customOAuthRoute.POST("/", controller.CreateCustomOAuthProvider)
			customOAuthRoute.PUT("/:id", controller.UpdateCustomOAuthProvider)
			customOAuthRoute.DELETE("/:id", controller.DeleteCustomOAuthProvider)
		}
		performanceRoute := apiRouter.Group("/performance")
		performanceRoute.Use(middleware.RootAuth())
		{
			performanceRoute.GET("/stats", controller.GetPerformanceStats)
			performanceRoute.DELETE("/disk_cache", controller.ClearDiskCache)
			performanceRoute.POST("/reset_stats", controller.ResetPerformanceStats)
			performanceRoute.POST("/gc", controller.ForceGC)
			performanceRoute.GET("/logs", controller.GetLogFiles)
			performanceRoute.DELETE("/logs", controller.CleanupLogFiles)
		}
		ratioSyncRoute := apiRouter.Group("/ratio_sync")
		ratioSyncRoute.Use(middleware.RootAuth())
		{
			ratioSyncRoute.GET("/channels", controller.GetSyncableChannels)
			ratioSyncRoute.POST("/fetch", controller.FetchUpstreamRatios)
		}
		channelRoute := apiRouter.Group("/channel")
		channelRoute.Use(middleware.AdminAuth())
		{
			channelRoute.GET("/", controller.GetAllChannels)
			channelRoute.GET("/search", controller.SearchChannels)
			channelRoute.GET("/models", controller.ChannelListModels)
			channelRoute.GET("/models_enabled", controller.EnabledListModels)
			channelRoute.GET("/probe-status", controller.GetChannelProbeStatus)
			channelRoute.POST("/probe/:id", controller.ProbeChannelNow)
			channelRoute.GET("/:id", controller.GetChannel)
			channelRoute.POST("/:id/key", middleware.RootAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.SecureVerificationRequired(), controller.GetChannelKey)
			channelRoute.GET("/test", controller.TestAllChannels)
			channelRoute.GET("/test/:id", controller.TestChannel)
			channelRoute.GET("/update_balance", controller.UpdateAllChannelsBalance)
			channelRoute.GET("/update_balance/:id", controller.UpdateChannelBalance)
			channelRoute.POST("/", controller.AddChannel)
			channelRoute.PUT("/", controller.UpdateChannel)
			channelRoute.DELETE("/disabled", controller.DeleteDisabledChannel)
			channelRoute.POST("/tag/disabled", controller.DisableTagChannels)
			channelRoute.POST("/tag/enabled", controller.EnableTagChannels)
			channelRoute.PUT("/tag", controller.EditTagChannels)
			channelRoute.DELETE("/:id", controller.DeleteChannel)
			channelRoute.POST("/batch", controller.DeleteChannelBatch)
			channelRoute.POST("/fix", controller.FixChannelsAbilities)
			channelRoute.GET("/fetch_models/:id", controller.FetchUpstreamModels)
			channelRoute.POST("/fetch_models", middleware.RootAuth(), controller.FetchModels)
			channelRoute.POST("/:id/codex/refresh", controller.RefreshCodexChannelCredential)
			channelRoute.GET("/:id/codex/usage", controller.GetCodexChannelUsage)
			channelRoute.POST("/ollama/pull", controller.OllamaPullModel)
			channelRoute.POST("/ollama/pull/stream", controller.OllamaPullModelStream)
			channelRoute.DELETE("/ollama/delete", controller.OllamaDeleteModel)
			channelRoute.GET("/ollama/version/:id", controller.OllamaVersion)
			channelRoute.POST("/batch/tag", controller.BatchSetChannelTag)
			channelRoute.GET("/tag/models", controller.GetTagModels)
			channelRoute.POST("/copy/:id", controller.CopyChannel)
			channelRoute.POST("/multi_key/manage", controller.ManageMultiKeys)
			channelRoute.POST("/upstream_updates/apply", controller.ApplyChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/apply_all", controller.ApplyAllChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/detect", controller.DetectChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/detect_all", controller.DetectAllChannelUpstreamModelUpdates)
		}
		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		{
			tokenRoute.GET("/", controller.GetAllTokens)
			tokenRoute.GET("/search", middleware.SearchRateLimit(), controller.SearchTokens)
			tokenRoute.GET("/:id", controller.GetToken)
			tokenRoute.GET("/:id/subscription-history", controller.GetSelfTokenSubscriptionHistory)
			tokenRoute.POST("/:id/key", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKey)
			tokenRoute.POST("/", controller.AddToken)
			tokenRoute.PUT("/", controller.UpdateToken)
			tokenRoute.DELETE("/:id", controller.DeleteToken)
			tokenRoute.POST("/batch", controller.DeleteTokenBatch)
			tokenRoute.POST("/batch/keys", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKeysBatch)
		}

		usageRoute := apiRouter.Group("/usage")
		usageRoute.Use(middleware.CORS())
		{
			tokenUsageRoute := usageRoute.Group("/token")
			tokenUsageRoute.Use(middleware.TokenAuthReadOnly(), middleware.TokenUsageRateLimit())
			{
				tokenUsageRoute.GET("/", controller.GetTokenUsage)
			}
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			redemptionRoute.GET("/", controller.GetAllRedemptions)
			redemptionRoute.GET("/search", controller.SearchRedemptions)
			redemptionRoute.GET("/:id", controller.GetRedemption)
			redemptionRoute.POST("/", controller.AddRedemption)
			redemptionRoute.PUT("/", controller.UpdateRedemption)
			redemptionRoute.DELETE("/invalid", controller.DeleteInvalidRedemption)
			redemptionRoute.DELETE("/:id", controller.DeleteRedemption)
		}
		logRoute := apiRouter.Group("/log")
		logRoute.GET("/", middleware.AdminAuth(), controller.GetAllLogs)
		logRoute.DELETE("/", middleware.AdminAuth(), controller.DeleteHistoryLogs)
		logRoute.GET("/stat", middleware.AdminAuth(), controller.GetLogsStat)
		logRoute.GET("/self/stat", middleware.UserAuth(), controller.GetLogsSelfStat)
		logRoute.GET("/channel_affinity_usage_cache", middleware.AdminAuth(), controller.GetChannelAffinityUsageCacheStats)
		logRoute.GET("/search", middleware.AdminAuth(), controller.SearchAllLogs)
		logRoute.GET("/self", middleware.UserAuth(), controller.GetUserLogs)
		logRoute.GET("/self/financial-flow", middleware.UserAuth(), controller.GetUserFinancialConsumeDaily)
		logRoute.GET("/self/search", middleware.UserAuth(), middleware.SearchRateLimit(), controller.SearchUserLogs)

		dataRoute := apiRouter.Group("/data")
		dataRoute.GET("/", middleware.AdminAuth(), controller.GetAllQuotaDates)
		dataRoute.GET("/users", middleware.AdminAuth(), controller.GetQuotaDatesByUser)
		dataRoute.GET("/self", middleware.UserAuth(), controller.GetUserQuotaDates)

		dashboardTrafficRoute := apiRouter.Group("/dashboard/traffic")
		dashboardTrafficRoute.GET("", middleware.AdminAuth(), controller.GetDashboardTraffic)
		dashboardTrafficRoute.GET("/self", middleware.UserAuth(), controller.GetDashboardTrafficSelf)

		usageStatisticsRoute := apiRouter.Group("/usage-statistics")
		usageStatisticsRoute.GET("/self", middleware.UserAuth(), middleware.UsageStatisticsRateLimit(), controller.GetUsageStatisticsSelf)
		usageStatisticsRoute.GET("/admin", middleware.RootAuth(), middleware.UsageStatisticsRateLimit(), controller.GetUsageStatisticsAdmin)
		usageStatisticsRoute.GET("/platform", middleware.UserAuth(), middleware.UsageStatisticsRateLimit(), controller.GetPlatformUsageOverview)

		logRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			logRoute.GET("/token", middleware.TokenAuthReadOnly(), controller.GetLogByKey)
		}
		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", controller.GetGroups)
		}

		prefillGroupRoute := apiRouter.Group("/prefill_group")
		prefillGroupRoute.Use(middleware.AdminAuth())
		{
			prefillGroupRoute.GET("/", controller.GetPrefillGroups)
			prefillGroupRoute.POST("/", controller.CreatePrefillGroup)
			prefillGroupRoute.PUT("/", controller.UpdatePrefillGroup)
			prefillGroupRoute.DELETE("/:id", controller.DeletePrefillGroup)
		}

		mjRoute := apiRouter.Group("/mj")
		mjRoute.GET("/self", middleware.UserAuth(), controller.GetUserMidjourney)
		mjRoute.GET("/", middleware.AdminAuth(), controller.GetAllMidjourney)

		taskRoute := apiRouter.Group("/task")
		{
			taskRoute.GET("/self", middleware.UserAuth(), controller.GetUserTask)
			taskRoute.GET("/", middleware.AdminAuth(), controller.GetAllTask)
		}

		vendorRoute := apiRouter.Group("/vendors")
		vendorRoute.Use(middleware.AdminAuth())
		{
			vendorRoute.GET("/", controller.GetAllVendors)
			vendorRoute.GET("/search", controller.SearchVendors)
			vendorRoute.GET("/:id", controller.GetVendorMeta)
			vendorRoute.POST("/", controller.CreateVendorMeta)
			vendorRoute.PUT("/", controller.UpdateVendorMeta)
			vendorRoute.DELETE("/:id", controller.DeleteVendorMeta)
		}

		modelsRoute := apiRouter.Group("/models")
		modelsRoute.Use(middleware.AdminAuth())
		{
			modelsRoute.GET("/sync_upstream/preview", controller.SyncUpstreamPreview)
			modelsRoute.POST("/sync_upstream", controller.SyncUpstreamModels)
			modelsRoute.GET("/missing", controller.GetMissingModels)
			modelsRoute.GET("/", controller.GetAllModelsMeta)
			modelsRoute.GET("/search", controller.SearchModelsMeta)
			modelsRoute.GET("/:id", controller.GetModelMeta)
			modelsRoute.POST("/", controller.CreateModelMeta)
			modelsRoute.PUT("/", controller.UpdateModelMeta)
			modelsRoute.DELETE("/:id", controller.DeleteModelMeta)
		}

		// Deployments (model deployment management)
		deploymentsRoute := apiRouter.Group("/deployments")
		deploymentsRoute.Use(middleware.AdminAuth())
		{
			deploymentsRoute.GET("/settings", controller.GetModelDeploymentSettings)
			deploymentsRoute.POST("/settings/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/", controller.GetAllDeployments)
			deploymentsRoute.GET("/search", controller.SearchDeployments)
			deploymentsRoute.POST("/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/hardware-types", controller.GetHardwareTypes)
			deploymentsRoute.GET("/locations", controller.GetLocations)
			deploymentsRoute.GET("/available-replicas", controller.GetAvailableReplicas)
			deploymentsRoute.POST("/price-estimation", controller.GetPriceEstimation)
			deploymentsRoute.GET("/check-name", controller.CheckClusterNameAvailability)
			deploymentsRoute.POST("/", controller.CreateDeployment)

			deploymentsRoute.GET("/:id", controller.GetDeployment)
			deploymentsRoute.GET("/:id/logs", controller.GetDeploymentLogs)
			deploymentsRoute.GET("/:id/containers", controller.ListDeploymentContainers)
			deploymentsRoute.GET("/:id/containers/:container_id", controller.GetContainerDetails)
			deploymentsRoute.PUT("/:id", controller.UpdateDeployment)
			deploymentsRoute.PUT("/:id/name", controller.UpdateDeploymentName)
			deploymentsRoute.POST("/:id/extend", controller.ExtendDeployment)
			deploymentsRoute.DELETE("/:id", controller.DeleteDeployment)
		}
	}
}
