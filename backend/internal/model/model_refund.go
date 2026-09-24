package model

import "time"

// RefundApplication 退款申请。每笔捐赠最多一条（donation_id 唯一约束），
// 重复提交/重复审核均幂等返回当前结果。
type RefundApplication struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	DonationID uint       `gorm:"uniqueIndex;not null" json:"donationId"`
	UserID     uint       `gorm:"index;not null" json:"userId"`
	Reason     string     `gorm:"size:255;not null" json:"reason"`
	Status     string     `gorm:"size:20;index;default:pending" json:"status"`
	ReviewerID uint       `json:"reviewerId"`
	ReviewNote string     `gorm:"size:255" json:"reviewNote"`
	ReviewedAt *time.Time `json:"reviewedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	Donation   *Donation  `gorm:"foreignKey:DonationID" json:"donation,omitempty"`
	User       *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}
