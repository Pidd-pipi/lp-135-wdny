package repository

import (
	"errors"
	"fmt"

	"github.com/givetrack/givetrack/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RefundRepository 退款申请数据访问。
type RefundRepository struct {
	db *gorm.DB
}

func NewRefundRepository(db *gorm.DB) *RefundRepository {
	return &RefundRepository{db: db}
}

// Create 创建退款申请。
func (r *RefundRepository) Create(refund *model.Refund) error {
	if err := r.db.Create(refund).Error; err != nil {
		return fmt.Errorf("create refund: %w", err)
	}
	return nil
}

// FindByDonationID 按捐赠记录查询退款申请。
func (r *RefundRepository) FindByDonationID(donationID uint) (*model.Refund, error) {
	var refund model.Refund
	err := r.db.Where("donation_id = ?", donationID).First(&refund).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by donation: %w", err)
	}
	return &refund, nil
}

// FindByID 按主键查询退款申请。
func (r *RefundRepository) FindByID(id uint) (*model.Refund, error) {
	var refund model.Refund
	err := r.db.Preload("Donation").Preload("Donation.Project").Preload("Donation.User").First(&refund, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by id: %w", err)
	}
	return &refund, nil
}

// FindByIDForUpdate 在事务内对退款申请加行锁查询，保证重复核准幂等。
func (r *RefundRepository) FindByIDForUpdate(id uint) (*model.Refund, error) {
	var refund model.Refund
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&refund, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund for update: %w", err)
	}
	return &refund, nil
}

// Update 更新退款申请。
func (r *RefundRepository) Update(refund *model.Refund) error {
	if err := r.db.Save(refund).Error; err != nil {
		return fmt.Errorf("update refund: %w", err)
	}
	return nil
}

// ListByUser 用户的退款申请（分页，最新优先）。
func (r *RefundRepository) ListByUser(userID uint, page, pageSize int) ([]model.Refund, int64, error) {
	var list []model.Refund
	var total int64
	q := r.db.Model(&model.Refund{}).Preload("Donation").Preload("Donation.Project").
		Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count refunds: %w", err)
	}
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list refunds by user: %w", err)
	}
	return list, total, nil
}

// ListPending 待审核退款申请。
func (r *RefundRepository) ListPending() ([]model.Refund, error) {
	var list []model.Refund
	if err := r.db.Preload("Donation").Preload("Donation.Project").Preload("Donation.User").
		Where("status = ?", "pending").
		Order("created_at ASC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list pending refunds: %w", err)
	}
	return list, nil
}
