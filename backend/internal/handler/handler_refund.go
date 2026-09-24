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

// Apply 捐款人提交退款申请（捐赠后 48 小时内）。
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
	refund, created, err := h.refundSvc.Apply(c.GetUint("user_id"), uint(id), req.Reason)
	if err != nil {
		util.FailError(c, err)
		return
	}
	if created {
		util.Created(c, gin.H{"message": "退款申请已提交，等待管理员审核", "refund": refund, "created": true})
		return
	}
	util.OK(c, gin.H{"message": "该笔捐赠已存在退款申请，返回当前处理结果", "refund": refund, "created": false})
}

// My 我的退款申请。
func (h *RefundHandler) My(c *gin.Context) {
	page, ps := util.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	if c.Query("limit") != "" {
		ps = atoi(c.Query("limit"))
	}
	list, total, err := h.refundSvc.MyRefunds(c.GetUint("user_id"), page, ps)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"refunds": list, "total": total, "page": page, "limit": ps})
}

// Pending 待审核退款申请（管理员）。
func (h *RefundHandler) Pending(c *gin.Context) {
	list, err := h.refundSvc.PendingRefunds()
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"refunds": list})
}

// Review 管理员审核退款申请（核准/驳回）。
func (h *RefundHandler) Review(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "invalid refund id")
		return
	}
	var req struct {
		Status  string `json:"status" binding:"required"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, 42200, err.Error())
		return
	}
	refund, changed, err := h.refundSvc.Review(c.GetUint("user_id"), uint(id), req.Status, req.Comment)
	if err != nil {
		util.FailError(c, err)
		return
	}
	message := "退款申请已核准，款项已扣回，电子凭证已失效"
	if req.Status == "rejected" {
		message = "退款申请已驳回"
	}
	if !changed {
		message = "该申请已处理，返回当前结果（金额未重复扣减）"
	}
	util.OK(c, gin.H{"message": message, "refund": refund, "changed": changed})
}
