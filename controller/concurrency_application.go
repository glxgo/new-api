package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type concurrencyApplicationRequest struct {
	RequestedLimit int    `json:"requested_limit"`
	Reason         string `json:"reason"`
	Contact        string `json:"contact"`
}

func CreateConcurrencyApplication(c *gin.Context) {
	var req concurrencyApplicationRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	req.Contact = strings.TrimSpace(req.Contact)
	if req.RequestedLimit < 1 || req.RequestedLimit > 10000 || len([]rune(req.Reason)) < 10 || len([]rune(req.Reason)) > 500 || len([]rune(req.Contact)) < 3 || len([]rune(req.Contact)) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请填写 10-500 字申请理由、有效联系方式，并将并发设置在 1-10000 之间"})
		return
	}
	application, err := model.CreateConcurrencyApplication(c.GetInt("id"), req.RequestedLimit, req.Reason, req.Contact)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": application})
}

func GetSelfConcurrencyApplications(c *gin.Context) {
	applications, total, err := model.ListConcurrencyApplications(c.GetInt("id"), "", 0, 20)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": applications, "total": total}})
}

func GetConcurrencyApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" && status != model.ConcurrencyApplicationPending && status != model.ConcurrencyApplicationApproved && status != model.ConcurrencyApplicationRejected {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid status"})
		return
	}
	applications, total, err := model.ListConcurrencyApplications(0, status, (page-1)*pageSize, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": applications, "total": total, "page": page, "page_size": pageSize}})
}

type concurrencyApplicationReviewRequest struct {
	Approve       bool   `json:"approve"`
	ApprovedLimit int    `json:"approved_limit"`
	AdminNote     string `json:"admin_note"`
}

func ReviewConcurrencyApplication(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid application id"})
		return
	}
	var req concurrencyApplicationReviewRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if len([]rune(strings.TrimSpace(req.AdminNote))) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "admin note is too long"})
		return
	}
	application, err := model.ReviewConcurrencyApplication(id, c.GetInt("id"), req.Approve, req.ApprovedLimit, req.AdminNote)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	recordManageAuditFor(c, application.UserId, "concurrency_application.review", map[string]interface{}{
		"application_id": id, "approved": req.Approve, "approved_limit": req.ApprovedLimit,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": application})
}
