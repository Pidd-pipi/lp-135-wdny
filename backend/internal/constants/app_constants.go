package constants

import "time"

// 应用常量。
const (
	AppName         = "givetrack"
	APIVersion      = "v1"
	PageDefault     = 1
	PageSizeDefault = 10
	PageSizeMax     = 100
)

// 角色
const (
	RoleUser  = "user"
	RoleOrg   = "org"
	RoleAdmin = "admin"
)

// 项目状态
const (
	ProjectPending   = "pending"
	ProjectApproved  = "approved"
	ProjectRejected  = "rejected"
	ProjectCompleted = "completed"
)

// 组织审核状态
const (
	OrgPending  = "pending"
	OrgApproved = "approved"
	OrgRejected = "rejected"
)

// 支付状态
const (
	PaymentSuccess  = "success"
	PaymentPending  = "pending"
	PaymentFailed   = "failed"
	PaymentRefunded = "refunded" // 已退款（电子凭证随之失效）
)

// 退款申请状态
const (
	RefundPending  = "pending"  // 待审核
	RefundApproved = "approved" // 已核准（金额已扣回）
	RefundRejected = "rejected" // 已驳回
)

// RefundWindow 捐款后可提交退款申请的时间窗口（48 小时）。
const RefundWindow time.Duration = 48 * time.Hour

// 项目分类
const (
	CategoryEducation   = "education"
	CategoryElderly     = "elderly"
	CategoryMedical     = "medical"
	CategoryDisaster    = "disaster"
	CategoryEnvironment = "environment"
	CategoryOther       = "other"
)

// 错误码
const (
	CodeOK              = 0
	CodeBadRequest      = 40000
	CodeUnauthorized    = 40100
	CodeForbidden       = 40300
	CodeNotFound        = 40400
	CodeConflict        = 40900
	CodeGone            = 41000
	CodeInternalError   = 50000
	CodeValidation      = 42200
	CodeTooManyRequests = 42900
)
