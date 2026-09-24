package service

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
	"gorm.io/gorm"
)

// RefundService 退款申请服务。
type RefundService struct {
	db         *gorm.DB
	refundRepo *repository.RefundRepository
	logger     *slog.Logger
}

func NewRefundService(db *gorm.DB, refundRepo *repository.RefundRepository, logger *slog.Logger) *RefundService {
	return &RefundService{db: db, refundRepo: refundRepo, logger: logger}
}

// ApplyInput 退款申请入参。
type ApplyInput struct {
	Reason string `json:"reason" binding:"required"`
}

// ReviewInput 退款审核入参。
type ReviewInput struct {
	Status string `json:"status" binding:"required"`
	Note   string `json:"note"`
}

// refundWindowExpired 判断捐赠是否已超出退款申请窗口（两天）。
func refundWindowExpired(donatedAt, now time.Time) bool {
	return now.Sub(donatedAt) > constants.RefundWindowHours*time.Hour
}

// Apply 捐款人提交退款申请（捐款后两天内、需填写原因）。
// 幂等：同一笔捐赠重复提交时不新建记录，直接返回当前申请及其处理状态。
// 返回的 bool 表示本次是否新建了申请。
func (s *RefundService) Apply(userID, donationID uint, in ApplyInput) (*model.RefundApplication, bool, error) {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, false, fmt.Errorf("reason is required")
	}

	donationRepo := repository.NewDonationRepository(s.db)
	d, err := donationRepo.FindByID(donationID)
	if err != nil {
		return nil, false, err
	}
	if d.UserID != userID {
		return nil, false, fmt.Errorf("forbidden: donation belongs to another user")
	}

	existing, err := s.refundRepo.FindByDonationID(donationID)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		// 已提交过申请（含已核准/已驳回），只返回当前结果，不产生新记录。
		return existing, false, nil
	}

	if d.PaymentStatus != constants.PaymentSuccess {
		return nil, false, fmt.Errorf("donation is not refundable")
	}
	if refundWindowExpired(d.CreatedAt, time.Now()) {
		return nil, false, fmt.Errorf("refund window expired: apply within %d hours of donation", constants.RefundWindowHours)
	}

	a := &model.RefundApplication{
		DonationID: donationID,
		UserID:     userID,
		Reason:     reason,
		Status:     constants.RefundPending,
	}
	if err := s.refundRepo.Create(a); err != nil {
		return nil, false, err
	}
	s.logger.Info("refund application created", "refundId", a.ID, "donationId", donationID, "userId", userID)
	return a, true, nil
}

// Pending 待处理退款申请列表（管理员）。
func (s *RefundService) Pending() ([]model.RefundApplication, error) {
	return s.refundRepo.FindPending()
}

// Review 管理员核准/驳回退款申请。
// 核准时在同一事务内完成：申请状态更新、捐赠标记为已退款（电子凭证随之失效）、
// 项目已筹金额与个人累计捐赠扣回（排行榜基于个人累计，同步生效）；
// 项目若原本已筹满，扣回后回到筹款中。
// 幂等：已审核过的申请再次审核直接返回当前结果，金额不会重复扣减。
func (s *RefundService) Review(adminID, refundID uint, in ReviewInput) (*model.RefundApplication, error) {
	if in.Status != constants.RefundApproved && in.Status != constants.RefundRejected {
		return nil, fmt.Errorf("invalid review status")
	}

	var application *model.RefundApplication
	err := s.db.Transaction(func(tx *gorm.DB) error {
		refundRepo := repository.NewRefundRepository(tx)
		donationRepo := repository.NewDonationRepository(tx)
		projectRepo := repository.NewProjectRepository(tx)
		userRepo := repository.NewUserRepository(tx)

		a, err := refundRepo.FindByIDForUpdate(refundID)
		if err != nil {
			return err
		}
		if a.Status != constants.RefundPending {
			// 已核准/已驳回的申请：直接返回当前结果，不重复扣减。
			application = a
			return nil
		}

		now := time.Now()
		a.Status = in.Status
		a.ReviewerID = adminID
		a.ReviewNote = strings.TrimSpace(in.Note)
		a.ReviewedAt = &now

		if in.Status == constants.RefundApproved {
			d, err := donationRepo.FindByID(a.DonationID)
			if err != nil {
				return err
			}
			if d.PaymentStatus != constants.PaymentSuccess {
				return fmt.Errorf("donation already refunded")
			}
			d.PaymentStatus = constants.PaymentRefunded
			if err := donationRepo.Update(d); err != nil {
				return err
			}

			project, err := projectRepo.FindByID(d.ProjectID)
			if err != nil {
				return err
			}
			project.CurrentAmount -= d.Amount
			if project.CurrentAmount < 0 {
				project.CurrentAmount = 0
			}
			if project.Status == constants.ProjectCompleted && project.CurrentAmount < project.TargetAmount {
				// 扣回后不再筹满，项目回到筹款中。
				project.Status = constants.ProjectApproved
			}
			if err := projectRepo.Update(project); err != nil {
				return err
			}

			user, err := userRepo.FindByID(d.UserID)
			if err != nil {
				return err
			}
			user.TotalDonation -= d.Amount
			if user.TotalDonation < 0 {
				user.TotalDonation = 0
			}
			if err := userRepo.Update(user); err != nil {
				return err
			}
		}

		if err := refundRepo.Update(a); err != nil {
			return err
		}
		application = a
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info("refund application reviewed", "refundId", application.ID, "status", application.Status)
	// 重新加载（含捐赠/项目/捐款人关联），返回最新状态。
	return s.refundRepo.FindByID(application.ID)
}
