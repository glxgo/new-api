package controller

import (
	"fmt"
	"html"
	"net/mail"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	invoiceSubjectCompany  = "company"
	invoiceSubjectPersonal = "personal"
	invoiceSubjectOther    = "other"
)

type invoiceApplicationRequest struct {
	TopUpIds    []int  `json:"top_up_ids"`
	SubjectType string `json:"subject_type"`
	Title       string `json:"title"`
	TaxpayerId  string `json:"taxpayer_id"`
	Email       string `json:"email"`
}

func normalizeInvoiceSubject(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case invoiceSubjectCompany, "企业":
		return invoiceSubjectCompany, true
	case invoiceSubjectPersonal, "个人":
		return invoiceSubjectPersonal, true
	case invoiceSubjectOther, "其他":
		return invoiceSubjectOther, true
	default:
		return "", false
	}
}

func validateInvoiceEmail(value string) bool {
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	return err == nil && address.Address == strings.TrimSpace(value) && strings.Contains(address.Address, "@")
}

func invoiceSubjectLabel(value string) string {
	switch value {
	case invoiceSubjectCompany:
		return "企业"
	case invoiceSubjectPersonal:
		return "个人"
	default:
		return "其他"
	}
}

func invoiceApplicationResponse(application *model.InvoiceApplication) gin.H {
	if application == nil {
		return gin.H{}
	}
	return gin.H{
		"id": application.Id, "user_id": application.UserId,
		"subject_type": application.SubjectType, "subject_label": invoiceSubjectLabel(application.SubjectType),
		"title": application.Title, "taxpayer_id": application.TaxpayerId, "email": application.Email,
		"total_amount": application.TotalAmount, "currency": application.Currency,
		"status": application.Status, "remark": application.Remark,
		"handler_id": application.HandlerId, "handler_name": application.HandlerName,
		"handled_at": application.HandledAt, "created_at": application.CreatedAt, "updated_at": application.UpdatedAt,
		"orders": application.Orders,
	}
}

func GetInvoiceEligibleOrders(c *gin.Context) {
	orders, err := model.ListEligibleInvoiceTopUps(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, orders)
}

func GetUserInvoiceApplications(c *gin.Context) {
	applications, err := model.GetUserInvoiceApplications(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(applications))
	for _, application := range applications {
		items = append(items, invoiceApplicationResponse(application))
	}
	common.ApiSuccess(c, items)
}

func CreateInvoiceApplication(c *gin.Context) {
	var req invoiceApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	subjectType, ok := normalizeInvoiceSubject(req.SubjectType)
	if !ok {
		common.ApiErrorMsg(c, "主体类型无效")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" || len([]rune(title)) > 120 {
		common.ApiErrorMsg(c, "请填写有效的发票抬头")
		return
	}
	taxpayerId := strings.TrimSpace(req.TaxpayerId)
	if subjectType == invoiceSubjectCompany && taxpayerId == "" {
		common.ApiErrorMsg(c, "企业主体必须填写纳税人识别号")
		return
	}
	if len([]rune(taxpayerId)) > 64 {
		common.ApiErrorMsg(c, "纳税人识别号过长")
		return
	}
	email := strings.TrimSpace(req.Email)
	if !validateInvoiceEmail(email) {
		common.ApiErrorMsg(c, "请填写有效的发票接收邮箱")
		return
	}
	application, err := model.CreateInvoiceApplication(c.GetInt("id"), subjectType, title, taxpayerId, email, req.TopUpIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	go notifyInvoiceApplication(application)
	common.ApiSuccess(c, invoiceApplicationResponse(application))
}

func AdminListInvoiceApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := strings.TrimSpace(c.DefaultQuery("status", "all"))
	applications, total, err := model.GetAllInvoiceApplications(status, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(applications))
	for _, application := range applications {
		items = append(items, invoiceApplicationResponse(application))
	}
	common.ApiSuccess(c, gin.H{"data": items, "total": total})
}

func AdminApproveInvoiceApplication(c *gin.Context) {
	reviewInvoiceApplication(c, model.InvoiceStatusApproved)
}

func AdminRejectInvoiceApplication(c *gin.Context) {
	reviewInvoiceApplication(c, model.InvoiceStatusRejected)
}

func reviewInvoiceApplication(c *gin.Context, status string) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Remark string `json:"remark"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiErrorMsg(c, "参数错误")
			return
		}
	}
	application, err := model.ReviewInvoiceApplication(id, status, c.GetInt("id"), c.GetString("username"), req.Remark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	go notifyInvoiceStatus(application)
	common.ApiSuccess(c, invoiceApplicationResponse(application))
}

func notifyInvoiceApplication(application *model.InvoiceApplication) {
	if application == nil {
		return
	}
	content := fmt.Sprintf(
		"<p>用户 ID <b>%d</b> 提交了发票申请。</p><p>主体：%s<br>抬头：%s<br>纳税人识别号：%s<br>接收邮箱：%s<br>合计：%.2f %s</p><p>订单明细：%s</p>",
		application.UserId, html.EscapeString(invoiceSubjectLabel(application.SubjectType)), html.EscapeString(application.Title), html.EscapeString(application.TaxpayerId), html.EscapeString(application.Email), application.TotalAmount, html.EscapeString(application.Currency), html.EscapeString(model.FormatInvoiceApplicationSummary(application)),
	)
	if err := common.SendEmail(fmt.Sprintf("[%s] 新发票申请 #%d", common.SystemName, application.Id), withdrawNotificationRecipients(), content); err != nil {
		common.SysError("send invoice application notify email failed: " + err.Error())
	}
}

func notifyInvoiceStatus(application *model.InvoiceApplication) {
	if application == nil {
		return
	}
	user, err := model.GetUserById(application.UserId, false)
	if err != nil || user == nil {
		common.SysError(fmt.Sprintf("load invoice applicant %d failed: %v", application.UserId, err))
		return
	}
	statusLabel := "已通过"
	if application.Status == model.InvoiceStatusRejected {
		statusLabel = "已拒绝"
	}
	remark := strings.TrimSpace(application.Remark)
	if remark == "" {
		remark = "无"
	}
	content := fmt.Sprintf("<p>您的发票申请 #%d %s。</p><p>抬头：%s<br>金额：%.2f %s<br>审核备注：%s</p>", application.Id, statusLabel, html.EscapeString(application.Title), application.TotalAmount, html.EscapeString(application.Currency), html.EscapeString(remark))
	if strings.TrimSpace(application.Email) != "" {
		if mailErr := common.SendEmail(fmt.Sprintf("[%s] 发票申请%s", common.SystemName, statusLabel), application.Email, content); mailErr != nil {
			common.SysError("send invoice status email failed: " + mailErr.Error())
		}
	}
	setting := user.GetSetting()
	notificationEmail := strings.TrimSpace(setting.NotificationEmail)
	if notificationEmail == "" {
		notificationEmail = strings.TrimSpace(user.Email)
	}
	if setting.NotifyType != dto.NotifyTypeEmail || !strings.EqualFold(notificationEmail, strings.TrimSpace(application.Email)) {
		if notifyErr := service.NotifyUser(user.Id, user.Email, setting, dto.NewNotify(dto.NotifyTypeInvoice, fmt.Sprintf("发票申请%s", statusLabel), content, nil)); notifyErr != nil {
			common.SysError("send invoice user notification failed: " + notifyErr.Error())
		}
	}
	logger.LogInfo(nil, fmt.Sprintf("invoice application %d marked %s", application.Id, application.Status))
}
