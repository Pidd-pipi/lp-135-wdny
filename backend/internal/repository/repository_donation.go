package repository

import (
	"errors"
	"fmt"

	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DonationRepository 捐赠数据访问。
type DonationRepository struct {
	db *gorm.DB
}

func NewDonationRepository(db *gorm.DB) *DonationRepository {
	return &DonationRepository{db: db}
}

func (r *DonationRepository) Create(d *model.Donation) error {
	if err := r.db.Create(d).Error; err != nil {
		return fmt.Errorf("create donation: %w", err)
	}
	return nil
}

func (r *DonationRepository) FindByID(id uint) (*model.Donation, error) {
	var d model.Donation
	err := r.db.Preload("Project").Preload("User").First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find donation by id: %w", err)
	}
	return &d, nil
}

// ListByProject 项目捐赠记录（成功支付，最新 20 条）。
func (r *DonationRepository) ListByProject(projectID uint, limit int) ([]model.Donation, error) {
	var list []model.Donation
	if err := r.db.Preload("User").Where("project_id = ? AND payment_status = ?", projectID, "success").
		Order("created_at DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list donations by project: %w", err)
	}
	return list, nil
}

// ListByUser 用户捐赠记录（分页）。
// 包含已退款记录，便于用户在个人中心查看退款处理状态。
func (r *DonationRepository) ListByUser(userID uint, page, pageSize int) ([]model.Donation, int64, error) {
	var list []model.Donation
	var total int64
	q := r.db.Model(&model.Donation{}).Preload("Project").Preload("Refund").
		Where("user_id = ? AND payment_status IN ?", userID, []string{constants.PaymentSuccess, constants.PaymentRefunded})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count donations: %w", err)
	}
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list donations by user: %w", err)
	}
	return list, total, nil
}

// FindByIDForUpdate 在事务内对捐赠记录加行锁查询，保证退款核准时金额只扣一次。
func (r *DonationRepository) FindByIDForUpdate(id uint) (*model.Donation, error) {
	var d model.Donation
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find donation for update: %w", err)
	}
	return &d, nil
}

// UpdateStatus 更新捐赠支付状态（用于退款核准时置为 refunded）。
func (r *DonationRepository) UpdateStatus(id uint, status string) error {
	if err := r.db.Model(&model.Donation{}).Where("id = ?", id).Update("payment_status", status).Error; err != nil {
		return fmt.Errorf("update donation status: %w", err)
	}
	return nil
}

// AdminReviewRepository 审核记录数据访问。
type AdminReviewRepository struct {
	db *gorm.DB
}

func NewAdminReviewRepository(db *gorm.DB) *AdminReviewRepository {
	return &AdminReviewRepository{db: db}
}

func (r *AdminReviewRepository) Create(a *model.AdminReview) error {
	if err := r.db.Create(a).Error; err != nil {
		return fmt.Errorf("create admin review: %w", err)
	}
	return nil
}
