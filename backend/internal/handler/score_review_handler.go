package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/dto"
	"github.com/onlineexam/onlineexam/internal/middleware"
	"github.com/onlineexam/onlineexam/internal/service"
	"github.com/onlineexam/onlineexam/internal/util"
)

// ScoreReviewHandler 成绩复核 HTTP 处理器。
type ScoreReviewHandler struct {
	svc    *service.ScoreReviewService
	recSvc *service.ExamRecordService
	logger *slog.Logger
}

// NewScoreReviewHandler 构造成绩复核处理器。
func NewScoreReviewHandler(svc *service.ScoreReviewService, recSvc *service.ExamRecordService, logger *slog.Logger) *ScoreReviewHandler {
	return &ScoreReviewHandler{svc: svc, recSvc: recSvc, logger: logger}
}

// Create 学生发起成绩复核。
func (h *ScoreReviewHandler) Create(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：recordId 参数非法"))
		return
	}
	var req dto.CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, util.WrapAppError(constants.CodeValidationFailed,
			fmt.Sprintf("成绩复核模块：发起复核参数校验失败（字段 reason）（%s）", err.Error()), err))
		return
	}
	claims := middleware.GetClaims(c)
	rv, err := h.svc.Create(c.Request.Context(), id, middleware.GetUserID(c), claims.Name, claims.Role, req.Reason)
	if err != nil {
		Error(c, err)
		return
	}
	SuccessMessage(c, constants.MsgReviewCreated, dto.ToReviewResponse(rv))
}

// Decide 教师驳回/受理复核。
func (h *ScoreReviewHandler) Decide(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：reviewId 参数非法"))
		return
	}
	var req dto.ReviewDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, util.WrapAppError(constants.CodeValidationFailed,
			fmt.Sprintf("成绩复核模块：处理复核参数校验失败（字段 action/comment/corrected_score）（%s）", err.Error()), err))
		return
	}
	claims := middleware.GetClaims(c)
	rv, rec, err := h.svc.Decide(c.Request.Context(), id, middleware.GetUserID(c), claims.Name, claims.Role, req)
	if err != nil {
		Error(c, err)
		return
	}
	msg := constants.MsgReviewRejected
	if req.Action == constants.ReviewActionApprove {
		msg = constants.MsgReviewApproved
	}
	SuccessMessage(c, msg, gin.H{
		"review": dto.ToReviewResponse(rv),
		"record": dto.ToRecordResponse(rec),
	})
}

// Get 查询单条复核（学生仅能查本人，教师/管理员可查任意）。
func (h *ScoreReviewHandler) Get(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：reviewId 参数非法"))
		return
	}
	rv, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}
	claims := middleware.GetClaims(c)
	if claims.Role == constants.RoleStudent && rv.StudentID.Hex() != claims.UserID {
		Error(c, util.NewAppError(constants.CodeReviewForbidden,
			fmt.Sprintf(constants.MsgReviewForbidden, claims.Role, rv.RecordID.Hex())))
		return
	}
	Success(c, dto.ToReviewResponse(rv))
}

// GetByRecord 按答卷查询复核（学生仅能查本人；学生成绩页与教师批改页复用）。
func (h *ScoreReviewHandler) GetByRecord(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：recordId 参数非法"))
		return
	}
	rec, err := h.recSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}
	claims := middleware.GetClaims(c)
	if claims.Role == constants.RoleStudent && rec.StudentID.Hex() != claims.UserID {
		Error(c, util.NewAppError(constants.CodeReviewForbidden,
			fmt.Sprintf(constants.MsgReviewForbidden, claims.Role, id.Hex())))
		return
	}
	rv, err := h.svc.GetByRecordID(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}
	Success(c, dto.ToReviewResponse(rv))
}

// ListMine 学生查询本人复核申请。
func (h *ScoreReviewHandler) ListMine(c *gin.Context) {
	page := util.GetPageParams(c, 10)
	list, total, err := h.svc.ListByStudent(c.Request.Context(), middleware.GetUserID(c), page.Page, page.PageSize)
	if err != nil {
		Error(c, err)
		return
	}
	items := make([]dto.ReviewResponse, 0, len(list))
	for _, rv := range list {
		items = append(items, dto.ToReviewResponse(rv))
	}
	PageResult(c, items, total, page.Page, page.PageSize)
}

// List 教师查询复核列表（默认待处理）。
func (h *ScoreReviewHandler) List(c *gin.Context) {
	page := util.GetPageParams(c, 10)
	status := c.Query("status")
	list, total, err := h.svc.List(c.Request.Context(), status, page.Page, page.PageSize)
	if err != nil {
		Error(c, err)
		return
	}
	items := make([]dto.ReviewResponse, 0, len(list))
	for _, rv := range list {
		items = append(items, dto.ToReviewResponse(rv))
	}
	PageResult(c, items, total, page.Page, page.PageSize)
}
