package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
	"gorm.io/gorm"
)

// RefundService 退款申请服务。
type RefundService struct {
	db           *gorm.DB
	refundRepo   *repository.RefundRepository
	donationRepo *repository.DonationRepository
	projectRepo  *repository.ProjectRepository
	userRepo     *repository.UserRepository
	logger       *slog.Logger
}

func NewRefundService(
	db *gorm.DB,
	refundRepo *repository.RefundRepository,
	donationRepo *repository.DonationRepository,
	projectRepo *repository.ProjectRepository,
	userRepo *repository.UserRepository,
	logger *slog.Logger,
) *RefundService {
	return &RefundService{
		db:           db,
		refundRepo:   refundRepo,
		donationRepo: donationRepo,
		projectRepo:  projectRepo,
		userRepo:     userRepo,
		logger:       logger,
	}
}

// 退款业务哨兵错误。
var (
	ErrRefundWindowExpired = errors.New("退款申请仅限捐赠完成后 48 小时内提交")
	ErrRefundAlreadyMoved  = errors.New("该笔捐赠已退款，无法再次申请")
	ErrRefundNotPending    = errors.New("退款申请已处理，无法重复审核")
	ErrRefundInvalidStatus = errors.New("invalid refund review status")
	errAlreadyReviewed     = errors.New("refund already reviewed")
)

// ApplyInput 退款申请入参。
type ApplyInput struct {
	Reason string `json:"reason" binding:"required,min=2,max=500"`
}

// Apply 捐款人提交退款申请。
// 同一笔捐赠只允许一条申请：已存在时直接返回当前结果（幂等，不重复创建）。
func (s *RefundService) Apply(userID, donationID uint, reason string) (*model.Refund, bool, error) {
	donation, err := s.donationRepo.FindByID(donationID)
	if err != nil {
		return nil, false, err
	}
	if donation.UserID != userID {
		return nil, false, fmt.Errorf("forbidden: donation belongs to another user")
	}
	// 已有申请（含待审核/已核准/已驳回）：直接返回当前结果，不做任何变更。
	if existing, err := s.refundRepo.FindByDonationID(donationID); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, false, err
	}
	if donation.PaymentStatus == constants.PaymentRefunded {
		return nil, false, ErrRefundAlreadyMoved
	}
	if donation.PaymentStatus != constants.PaymentSuccess {
		return nil, false, fmt.Errorf("donation is not refundable")
	}
	if time.Since(donation.CreatedAt) > constants.RefundWindow {
		return nil, false, ErrRefundWindowExpired
	}

	refund := &model.Refund{
		DonationID: donationID,
		UserID:     userID,
		Reason:     reason,
		Status:     constants.RefundPending,
	}
	if err := s.refundRepo.Create(refund); err != nil {
		return nil, false, err
	}
	s.logger.Info("refund applied", "refundId", refund.ID, "donationId", donationID, "userId", userID)
	return refund, true, nil
}

// Review 管理员审核退款申请。
// 核准时在同一事务内将款项从项目进度、个人累计（即排行榜口径）扣回，
// 项目原本已筹满的回退到筹款中，捐赠置为 refunded 使电子凭证失效。
// 已核准的申请重复提交只返回当前结果，金额不会再扣一次。
func (s *RefundService) Review(adminID, refundID uint, status, comment string) (*model.Refund, bool, error) {
	if status != constants.RefundApproved && status != constants.RefundRejected {
		return nil, false, ErrRefundInvalidStatus
	}

	refund, err := s.refundRepo.FindByID(refundID)
	if err != nil {
		return nil, false, err
	}
	// 已处理过的申请：幂等返回当前结果。
	if refund.Status != constants.RefundPending {
		return refund, false, nil
	}

	if status == constants.RefundRejected {
		now := time.Now()
		refund.Status = constants.RefundRejected
		refund.ReviewerID = adminID
		refund.ReviewComment = comment
		refund.ReviewedAt = &now
		if err := s.refundRepo.Update(refund); err != nil {
			return nil, false, err
		}
		s.logger.Info("refund rejected", "refundId", refundID, "adminId", adminID)
		return refund, true, nil
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		refundRepo := repository.NewRefundRepository(tx)
		donationRepo := repository.NewDonationRepository(tx)
		projectRepo := repository.NewProjectRepository(tx)
		userRepo := repository.NewUserRepository(tx)

		// 行锁锁定申请，并发核准时只有一个请求能看到 pending。
		locked, err := refundRepo.FindByIDForUpdate(refundID)
		if err != nil {
			return err
		}
		if locked.Status != constants.RefundPending {
			// 已被其他请求处理：后续以幂等结果返回，不再扣减。
			*refund = *locked
			return errAlreadyReviewed
		}

		donation, err := donationRepo.FindByIDForUpdate(refund.DonationID)
		if err != nil {
			return err
		}
		if donation.PaymentStatus == constants.PaymentRefunded {
			return errAlreadyReviewed
		}

		// 1. 扣回项目进度；原本已筹满的项目回到筹款中。
		project, err := projectRepo.FindByIDForUpdate(donation.ProjectID)
		if err != nil {
			return err
		}
		project.CurrentAmount -= donation.Amount
		if project.CurrentAmount < 0 {
			project.CurrentAmount = 0
		}
		if project.Status == constants.ProjectCompleted && project.CurrentAmount < project.TargetAmount {
			project.Status = constants.ProjectApproved
		}
		if err := projectRepo.Update(project); err != nil {
			return err
		}

		// 2. 扣回个人累计（排行榜以此字段排序，随之更新）。
		donor, err := userRepo.FindByIDForUpdate(donation.UserID)
		if err != nil {
			return err
		}
		donor.TotalDonation -= donation.Amount
		if donor.TotalDonation < 0 {
			donor.TotalDonation = 0
		}
		if err := userRepo.Update(donor); err != nil {
			return err
		}

		// 3. 捐赠置为已退款，电子凭证随即失效。
		if err := donationRepo.UpdateStatus(donation.ID, constants.PaymentRefunded); err != nil {
			return err
		}

		// 4. 申请标记为已核准。
		now := time.Now()
		locked.Status = constants.RefundApproved
		locked.ReviewerID = adminID
		locked.ReviewComment = comment
		locked.ReviewedAt = &now
		if err := refundRepo.Update(locked); err != nil {
			return err
		}
		*refund = *locked
		return nil
	})
	if errors.Is(err, errAlreadyReviewed) {
		return refund, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	s.logger.Info("refund approved", "refundId", refundID, "donationId", refund.DonationID, "adminId", adminID)
	return refund, true, nil
}

// MyRefunds 我的退款申请列表。
func (s *RefundService) MyRefunds(userID uint, page, pageSize int) ([]model.Refund, int64, error) {
	return s.refundRepo.ListByUser(userID, page, pageSize)
}

// PendingRefunds 待审核退款申请。
func (s *RefundService) PendingRefunds() ([]model.Refund, error) {
	return s.refundRepo.ListPending()
}
