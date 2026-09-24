package service

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newRefundTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Organization{}, &model.Project{},
		&model.ProjectUpdate{}, &model.Donation{}, &model.Refund{},
		&model.AdminReview{}, &model.VolunteerService{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 内存库在进程内共享，逐表清空保证用例隔离。
	if err := db.Exec(`DELETE FROM refunds; DELETE FROM donations; DELETE FROM projects;
		DELETE FROM organizations; DELETE FROM users; DELETE FROM admin_reviews; DELETE FROM project_updates;`).Error; err != nil {
		t.Fatalf("clean tables: %v", err)
	}
	return db
}

func setupRefundScenario(t *testing.T, db *gorm.DB, completed bool) (*RefundService, *DonationService, *model.User, *model.Project, *model.Donation) {
	t.Helper()
	logger := slog.Default()
	userRepo := repository.NewUserRepository(db)
	projectRepo := repository.NewProjectRepository(db)
	donationRepo := repository.NewDonationRepository(db)
	refundRepo := repository.NewRefundRepository(db)

	user := &model.User{Username: "refunduser", Email: "refund@x.cn", PasswordHash: "x", Role: "user", TotalDonation: 0}
	if err := userRepo.Create(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	orgUser := &model.User{Username: "orgowner", Email: "orgowner@x.cn", PasswordHash: "x", Role: "org"}
	if err := userRepo.Create(orgUser); err != nil {
		t.Fatalf("create org user: %v", err)
	}
	org := &model.Organization{UserID: orgUser.ID, Name: "org", Status: constants.OrgApproved}
	if err := db.Create(org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	target := 100.0
	status := constants.ProjectApproved
	project := &model.Project{OrganizationID: org.ID, Title: "p", Category: "other", TargetAmount: target, Status: status}
	if err := projectRepo.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	amount := 100.0
	if !completed {
		amount = 60 // 捐完后仍未筹满
	}
	d := &model.Donation{
		UserID: user.ID, ProjectID: project.ID, Amount: amount,
		PaymentStatus: constants.PaymentSuccess, CertificateNo: "CERT-TEST",
	}
	if err := donationRepo.Create(d); err != nil {
		t.Fatalf("create donation: %v", err)
	}
	project.CurrentAmount = amount
	if completed {
		project.Status = constants.ProjectCompleted
	}
	if err := projectRepo.Update(project); err != nil {
		t.Fatalf("update project: %v", err)
	}
	user.TotalDonation = amount
	if err := userRepo.Update(user); err != nil {
		t.Fatalf("update user: %v", err)
	}

	refundSvc := NewRefundService(db, refundRepo, donationRepo, projectRepo, userRepo, logger)
	donationSvc := NewDonationService(db, donationRepo, projectRepo, userRepo, logger)
	return refundSvc, donationSvc, user, project, d
}

// 完整流程：申请 -> 核准 -> 金额三处扣回 + 项目回筹款中 + 凭证失效；重复核准幂等。
func TestRefundApproveFlow(t *testing.T) {
	db := newRefundTestDB(t)
	refundSvc, donationSvc, user, project, d := setupRefundScenario(t, db, true)

	// 提交申请
	refund, created, err := refundSvc.Apply(user.ID, d.ID, "误操作捐款，申请退款")
	if err != nil || !created {
		t.Fatalf("apply refund: created=%v err=%v", created, err)
	}
	if refund.Status != constants.RefundPending {
		t.Fatalf("new refund status = %q, want pending", refund.Status)
	}

	// 重复提交：只返回已有申请，不新建
	again, created2, err := refundSvc.Apply(user.ID, d.ID, "另一个原因")
	if err != nil || created2 {
		t.Fatalf("duplicate apply should be idempotent: created=%v err=%v", created2, err)
	}
	if again.ID != refund.ID || again.Reason != "误操作捐款，申请退款" {
		t.Fatalf("duplicate apply should return existing refund unchanged")
	}

	// 核准
	reviewed, changed, err := refundSvc.Review(1, refund.ID, constants.RefundApproved, "")
	if err != nil || !changed {
		t.Fatalf("approve refund: changed=%v err=%v", changed, err)
	}
	if reviewed.Status != constants.RefundApproved || reviewed.ReviewerID != 1 || reviewed.ReviewedAt == nil {
		t.Fatalf("approved refund not recorded correctly: %+v", reviewed)
	}

	// 项目金额扣回，已筹满项目回到筹款中
	var p model.Project
	_ = db.First(&p, project.ID).Error
	if p.CurrentAmount != 0 {
		t.Fatalf("project current amount = %v, want 0", p.CurrentAmount)
	}
	if p.Status != constants.ProjectApproved {
		t.Fatalf("project status = %q, want approved (back to fundraising)", p.Status)
	}

	// 个人累计扣回（排行榜口径）
	var u model.User
	_ = db.First(&u, user.ID).Error
	if u.TotalDonation != 0 {
		t.Fatalf("user total donation = %v, want 0", u.TotalDonation)
	}

	// 捐赠变为已退款
	var got model.Donation
	_ = db.First(&got, d.ID).Error
	if got.PaymentStatus != constants.PaymentRefunded {
		t.Fatalf("donation status = %q, want refunded", got.PaymentStatus)
	}

	// 电子凭证失效
	if _, err := donationSvc.Certificate(user.ID, d.ID); err != ErrCertificateInvalid {
		t.Fatalf("certificate err = %v, want ErrCertificateInvalid", err)
	}

	// 再次核准：幂等，金额不再扣一次
	_, changed2, err := refundSvc.Review(1, refund.ID, constants.RefundApproved, "")
	if err != nil || changed2 {
		t.Fatalf("re-approve should be idempotent: changed=%v err=%v", changed2, err)
	}
	_ = db.First(&p, project.ID).Error
	if p.CurrentAmount != 0 || p.Status != constants.ProjectApproved {
		t.Fatalf("re-approve must not deduct again: amount=%v status=%q", p.CurrentAmount, p.Status)
	}
}

// 未筹满项目核准退款后状态保持 approved（筹款中），金额正确扣回。
func TestRefundApproveKeepsApprovedWhenNotCompleted(t *testing.T) {
	db := newRefundTestDB(t)
	refundSvc, _, user, project, d := setupRefundScenario(t, db, false)

	refund, _, err := refundSvc.Apply(user.ID, d.ID, "原因")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, _, err := refundSvc.Review(1, refund.ID, constants.RefundApproved, ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var p model.Project
	_ = db.First(&p, project.ID).Error
	if p.CurrentAmount != 0 {
		t.Fatalf("amount = %v, want 0", p.CurrentAmount)
	}
	if p.Status != constants.ProjectApproved {
		t.Fatalf("status = %q, want approved", p.Status)
	}
}

// 驳回不影响金额；驳回后不可再核准。
func TestRefundReject(t *testing.T) {
	db := newRefundTestDB(t)
	refundSvc, _, user, project, d := setupRefundScenario(t, db, true)
	refund, _, err := refundSvc.Apply(user.ID, d.ID, "原因")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	rejected, changed, err := refundSvc.Review(1, refund.ID, constants.RefundRejected, "材料不全")
	if err != nil || !changed || rejected.Status != constants.RefundRejected {
		t.Fatalf("reject: changed=%v status=%q err=%v", changed, rejected.Status, err)
	}
	var p model.Project
	_ = db.First(&p, project.ID).Error
	if p.CurrentAmount != 100 || p.Status != constants.ProjectCompleted {
		t.Fatalf("reject must not change money: amount=%v status=%q", p.CurrentAmount, p.Status)
	}
	// 已处理的申请再核准：只返回当前驳回结果，不扣钱
	r2, changed2, err := refundSvc.Review(1, refund.ID, constants.RefundApproved, "")
	if err != nil || changed2 || r2.Status != constants.RefundRejected {
		t.Fatalf("review after reject should stay idempotent: changed=%v status=%q err=%v", changed2, r2.Status, err)
	}
}

// 超过 48 小时不可申请。
func TestRefundWindowExpired(t *testing.T) {
	db := newRefundTestDB(t)
	refundSvc, _, user, _, d := setupRefundScenario(t, db, false)
	old := time.Now().Add(-constants.RefundWindow - time.Minute)
	if err := db.Model(&model.Donation{}).Where("id = ?", d.ID).Update("created_at", old).Error; err != nil {
		t.Fatalf("backdate donation: %v", err)
	}
	if _, _, err := refundSvc.Apply(user.ID, d.ID, "过期了"); err != ErrRefundWindowExpired {
		t.Fatalf("expected ErrRefundWindowExpired, got %v", err)
	}
}

// 不能为他人的捐赠申请退款。
func TestRefundApplyForbidden(t *testing.T) {
	db := newRefundTestDB(t)
	refundSvc, _, _, _, d := setupRefundScenario(t, db, false)
	other := &model.User{Username: "other", Email: "other@x.cn", PasswordHash: "x", Role: "user"}
	if err := db.Create(other).Error; err != nil {
		t.Fatalf("create other: %v", err)
	}
	if _, _, err := refundSvc.Apply(other.ID, d.ID, "不是我的"); err == nil {
		t.Fatalf("expected forbidden error")
	}
}
