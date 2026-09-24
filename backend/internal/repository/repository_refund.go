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

func (r *RefundRepository) Create(a *model.RefundApplication) error {
	if err := r.db.Create(a).Error; err != nil {
		return fmt.Errorf("create refund application: %w", err)
	}
	return nil
}

func (r *RefundRepository) FindByID(id uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.Preload("Donation").Preload("Donation.Project").Preload("User").First(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund application by id: %w", err)
	}
	return &a, nil
}

// FindByIDForUpdate 事务内加行锁查询，防止并发审核对同一申请重复扣减。
func (r *RefundRepository) FindByIDForUpdate(id uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund application by id: %w", err)
	}
	return &a, nil
}

// FindByDonationID 查询某笔捐赠的退款申请；不存在时返回 (nil, nil)。
func (r *RefundRepository) FindByDonationID(donationID uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.Where("donation_id = ?", donationID).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find refund application by donation: %w", err)
	}
	return &a, nil
}

// FindPending 待处理退款申请（按提交时间正序，先申请先处理）。
func (r *RefundRepository) FindPending() ([]model.RefundApplication, error) {
	var list []model.RefundApplication
	if err := r.db.Preload("Donation").Preload("Donation.Project").Preload("User").
		Where("status = ?", "pending").Order("created_at ASC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("find pending refunds: %w", err)
	}
	return list, nil
}

func (r *RefundRepository) Update(a *model.RefundApplication) error {
	if err := r.db.Save(a).Error; err != nil {
		return fmt.Errorf("update refund application: %w", err)
	}
	return nil
}

// ListByDonationIDs 批量查询捐赠对应的退款申请（用于个人捐赠记录填充处理状态）。
func (r *RefundRepository) ListByDonationIDs(donationIDs []uint) ([]model.RefundApplication, error) {
	if len(donationIDs) == 0 {
		return nil, nil
	}
	var list []model.RefundApplication
	if err := r.db.Where("donation_id IN ?", donationIDs).Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list refunds by donations: %w", err)
	}
	return list, nil
}
