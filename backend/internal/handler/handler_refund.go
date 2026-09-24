package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/givetrack/givetrack/internal/service"
	"github.com/givetrack/givetrack/internal/util"
)

// RefundHandler 退款申请处理器。
type RefundHandler struct {
	refundSvc *service.RefundService
}

func NewRefundHandler(refundSvc *service.RefundService) *RefundHandler {
	return &RefundHandler{refundSvc: refundSvc}
}

// Apply 捐款人提交退款申请（捐款后两天内，需填写原因）。
// 重复提交时幂等返回当前申请及处理状态。
func (h *RefundHandler) Apply(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "invalid donation id")
		return
	}
	var req service.ApplyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, 42200, err.Error())
		return
	}
	a, created, err := h.refundSvc.Apply(c.GetUint("user_id"), uint(id), req)
	if err != nil {
		util.FailError(c, err)
		return
	}
	message := "退款申请已提交"
	if !created {
		message = "已存在退款申请，返回当前处理结果"
	}
	util.OK(c, gin.H{"message": message, "refund": a})
}

// Pending 待处理退款申请列表（管理员）。
func (h *RefundHandler) Pending(c *gin.Context) {
	list, err := h.refundSvc.Pending()
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"refunds": list})
}

// Review 管理员核准/驳回退款申请。核准后金额从项目进度、个人累计与排行榜扣回。
func (h *RefundHandler) Review(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "invalid refund id")
		return
	}
	var req service.ReviewInput
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, 42200, err.Error())
		return
	}
	a, err := h.refundSvc.Review(c.GetUint("user_id"), uint(id), req)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"refund": a})
}
