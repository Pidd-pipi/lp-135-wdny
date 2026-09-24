package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/middleware"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
	"github.com/givetrack/givetrack/internal/service"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type refundHTTPEnv struct {
	router   *gin.Engine
	db       *gorm.DB
	authSvc  *service.AuthService
	donor    *model.User
	admin    *model.User
	donation *model.Donation
	project  *model.Project
}

func setupRefundHTTP(t *testing.T, completed bool) refundHTTPEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Organization{}, &model.Project{}, &model.ProjectUpdate{},
		&model.Donation{}, &model.Refund{}, &model.AdminReview{}, &model.VolunteerService{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	orgRepo := repository.NewOrganizationRepository(db)
	projectRepo := repository.NewProjectRepository(db)
	updateRepo := repository.NewProjectUpdateRepository(db)
	donationRepo := repository.NewDonationRepository(db)
	refundRepo := repository.NewRefundRepository(db)
	reviewRepo := repository.NewAdminReviewRepository(db)
	logger := slog.Default()

	authSvc := service.NewAuthService(userRepo, orgRepo, "test-secret-key-0123456789", 24, logger)
	_ = service.NewProjectService(projectRepo, updateRepo, orgRepo, donationRepo, logger)
	donationSvc := service.NewDonationService(db, donationRepo, projectRepo, userRepo, logger)
	refundSvc := service.NewRefundService(db, refundRepo, donationRepo, projectRepo, userRepo, logger)
	_ = service.NewRankingService(userRepo, logger)
	_ = service.NewAdminService(projectRepo, orgRepo, reviewRepo, logger)

	donor := &model.User{Username: "httpdonor", Email: "httpdonor@x.cn", PasswordHash: "x", Role: "user"}
	if err := userRepo.Create(donor); err != nil {
		t.Fatal(err)
	}
	adm := &model.User{Username: "httpadmin", Email: "httpadmin@x.cn", PasswordHash: "x", Role: "admin"}
	if err := userRepo.Create(adm); err != nil {
		t.Fatal(err)
	}
	orgUser := &model.User{Username: "httporg", Email: "httporg@x.cn", PasswordHash: "x", Role: "org"}
	if err := userRepo.Create(orgUser); err != nil {
		t.Fatal(err)
	}
	org := &model.Organization{UserID: orgUser.ID, Name: "org", Status: constants.OrgApproved}
	if err := db.Create(org).Error; err != nil {
		t.Fatal(err)
	}
	amount := 100.0
	status := constants.ProjectApproved
	if completed {
		status = constants.ProjectCompleted
	}
	project := &model.Project{OrganizationID: org.ID, Title: "p", Category: "other", TargetAmount: 100, CurrentAmount: amount, Status: status}
	if err := projectRepo.Create(project); err != nil {
		t.Fatal(err)
	}
	donation := &model.Donation{UserID: donor.ID, ProjectID: project.ID, Amount: amount, PaymentStatus: constants.PaymentSuccess, CertificateNo: "CERT-HTTP"}
	if err := donationRepo.Create(donation); err != nil {
		t.Fatal(err)
	}
	donor.TotalDonation = amount
	if err := userRepo.Update(donor); err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(middleware.Recovery(logger))
	v1 := r.Group("/api/v1")

	donH := NewDonationHandler(donationSvc)
	refH := NewRefundHandler(refundSvc)
	v1.POST("/donations/:id/refund", middleware.Auth(authSvc), refH.Apply)
	v1.GET("/donations/:id/certificate", middleware.Auth(authSvc), donH.Certificate)
	v1.GET("/refunds/my", middleware.Auth(authSvc), refH.My)
	adminG := v1.Group("/admin", middleware.Auth(authSvc), middleware.RequireRole(constants.RoleAdmin))
	adminG.GET("/refunds/pending", refH.Pending)
	adminG.POST("/refunds/:id/review", refH.Review)

	return refundHTTPEnv{router: r, db: db, authSvc: authSvc, donor: donor, admin: adm, donation: donation, project: project}
}

func doJSON(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func tokenFor(t *testing.T, env refundHTTPEnv, u *model.User) string {
	t.Helper()
	tok, err := env.authSvc.GenerateToken(u)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// 完整 HTTP 流程：申请 -> 重复申请幂等 -> 管理员核准 -> 凭证 410 -> 重复核准不再扣减。
func TestRefundHTTPEndToEnd(t *testing.T) {
	env := setupRefundHTTP(t, true)
	donorTok := tokenFor(t, env, env.donor)
	adminTok := tokenFor(t, env, env.admin)

	// 未登录 401
	if w := doJSON(t, env.router, "POST", "/api/v1/donations/"+fmt.Sprint(env.donation.ID)+"/refund", "", map[string]string{"reason": "误操作"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("no token: status=%d want 401", w.Code)
	}

	// 提交申请 201
	w := doJSON(t, env.router, "POST", "/api/v1/donations/"+fmt.Sprint(env.donation.ID)+"/refund", donorTok, map[string]string{"reason": "误操作申请退款"})
	if w.Code != http.StatusCreated {
		t.Fatalf("apply status=%d body=%s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			Created bool         `json:"created"`
			Refund  model.Refund `json:"refund"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if !created.Data.Created || created.Data.Refund.Status != constants.RefundPending {
		t.Fatalf("unexpected apply response: %s", w.Body.String())
	}
	refundID := created.Data.Refund.ID

	// 重复提交：200 + created=false，幂等
	w = doJSON(t, env.router, "POST", "/api/v1/donations/"+fmt.Sprint(env.donation.ID)+"/refund", donorTok, map[string]string{"reason": "再试一次"})
	if w.Code != http.StatusOK {
		t.Fatalf("duplicate apply status=%d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"created":false`)) {
		t.Fatalf("duplicate apply should be created=false: %s", w.Body.String())
	}

	// 我的退款列表
	w = doJSON(t, env.router, "GET", "/api/v1/refunds/my?limit=10", donorTok, nil)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("误操作申请退款")) {
		t.Fatalf("my refunds: status=%d body=%s", w.Code, w.Body.String())
	}

	// 普通用户不能访问管理员接口
	w = doJSON(t, env.router, "GET", "/api/v1/admin/refunds/pending", donorTok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("donor admin access: status=%d want 403", w.Code)
	}

	// 管理员待处理列表
	w = doJSON(t, env.router, "GET", "/api/v1/admin/refunds/pending", adminTok, nil)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"status":"pending"`)) {
		t.Fatalf("pending list: status=%d body=%s", w.Code, w.Body.String())
	}

	// 核准前凭证可用
	if w := doJSON(t, env.router, "GET", "/api/v1/donations/"+fmt.Sprint(env.donation.ID)+"/certificate", donorTok, nil); w.Code != http.StatusOK {
		t.Fatalf("certificate before refund: status=%d", w.Code)
	}

	// 核准
	w = doJSON(t, env.router, "POST", "/api/v1/admin/refunds/"+fmt.Sprint(refundID)+"/review", adminTok, map[string]string{"status": "approved"})
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"changed":true`)) {
		t.Fatalf("approve: status=%d body=%s", w.Code, w.Body.String())
	}

	// 凭证随即失效 410
	w = doJSON(t, env.router, "GET", "/api/v1/donations/"+fmt.Sprint(env.donation.ID)+"/certificate", donorTok, nil)
	if w.Code != http.StatusGone {
		t.Fatalf("certificate after refund: status=%d want 410, body=%s", w.Code, w.Body.String())
	}

	// 重复核准：changed=false，不再次扣减
	w = doJSON(t, env.router, "POST", "/api/v1/admin/refunds/"+fmt.Sprint(refundID)+"/review", adminTok, map[string]string{"status": "approved"})
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"changed":false`)) {
		t.Fatalf("re-approve: status=%d body=%s", w.Code, w.Body.String())
	}

	// 数据库断言：金额三处扣回、项目回退
	var p model.Project
	if err := env.db.First(&p, env.project.ID).Error; err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if p.CurrentAmount != 0 || p.Status != constants.ProjectApproved {
		t.Fatalf("project after refund: amount=%v status=%q", p.CurrentAmount, p.Status)
	}
	var u model.User
	if err := env.db.First(&u, env.donor.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if u.TotalDonation != 0 {
		t.Fatalf("user total donation = %v, want 0 (ranking source deducted)", u.TotalDonation)
	}
	var d model.Donation
	if err := env.db.First(&d, env.donation.ID).Error; err != nil {
		t.Fatalf("reload donation: %v", err)
	}
	if d.PaymentStatus != constants.PaymentRefunded {
		t.Fatalf("donation status = %q, want refunded", d.PaymentStatus)
	}
}

// 超过 48 小时提交返回业务错误。
func TestRefundHTTPWindowExpired(t *testing.T) {
	env := setupRefundHTTP(t, false)
	old := env.donation.CreatedAt.Add(-constants.RefundWindow).Add(-time.Minute)
	if err := env.db.Model(&model.Donation{}).Where("id = ?", env.donation.ID).Update("created_at", old).Error; err != nil {
		t.Fatal(err)
	}
	tok := tokenFor(t, env, env.donor)
	w := doJSON(t, env.router, "POST", "/api/v1/donations/"+fmt.Sprint(env.donation.ID)+"/refund", tok, map[string]string{"reason": "过期"})
	if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte("48")) {
		t.Fatalf("expired window: status=%d body=%s", w.Code, w.Body.String())
	}
}
