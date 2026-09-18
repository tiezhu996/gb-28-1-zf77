package handler

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
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
	logger *slog.Logger
}

// NewScoreReviewHandler 构造成绩复核处理器。
func NewScoreReviewHandler(svc *service.ScoreReviewService, logger *slog.Logger) *ScoreReviewHandler {
	return &ScoreReviewHandler{svc: svc, logger: logger}
}

// operatorFromClaims 从 JWT 组装复核操作人。
func operatorFromClaims(c *gin.Context) service.Operator {
	claims := middleware.GetClaims(c)
	name := ""
	role := ""
	if claims != nil {
		name = claims.Name
		role = claims.Role
	}
	return service.Operator{ID: middleware.GetUserID(c), Name: name, Role: role}
}

// Create 学生对本人答卷发起一次成绩复核（批改完成 48h 内，说明理由）。
func (h *ScoreReviewHandler) Create(c *gin.Context) {
	recordID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：recordId 参数非法（角色 student）"))
		return
	}
	var req dto.CreateScoreReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, util.WrapAppError(constants.CodeValidationFailed, fmt.Sprintf("成绩复核模块：发起复核参数校验失败（字段 reason）（%s）", err.Error()), err))
		return
	}
	rev, err := h.svc.Create(c.Request.Context(), recordID, req.Reason, operatorFromClaims(c))
	if err != nil {
		Error(c, err)
		return
	}
	SuccessMessage(c, constants.MsgReviewCreated, dto.ToScoreReviewResponse(rev))
}

// Approve 教师受理复核并更正总分/及格状态（必须填写意见）。
func (h *ScoreReviewHandler) Approve(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：id 参数非法（角色 teacher）"))
		return
	}
	var req dto.HandleScoreReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, util.WrapAppError(constants.CodeValidationFailed, fmt.Sprintf("成绩复核模块：受理复核参数校验失败（字段 opinion/corrected_score）（%s）", err.Error()), err))
		return
	}
	req.Action = "approve"
	rev, err := h.svc.Process(c.Request.Context(), id, req, operatorFromClaims(c))
	if err != nil {
		Error(c, err)
		return
	}
	SuccessMessage(c, constants.MsgReviewApproved, dto.ToScoreReviewResponse(rev))
}

// Reject 教师驳回复核（必须填写意见，不得改分）。
func (h *ScoreReviewHandler) Reject(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：id 参数非法（角色 teacher）"))
		return
	}
	var req dto.HandleScoreReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, util.WrapAppError(constants.CodeValidationFailed, fmt.Sprintf("成绩复核模块：驳回复核参数校验失败（字段 opinion）（%s）", err.Error()), err))
		return
	}
	req.Action = "reject"
	req.CorrectedScore = nil
	rev, err := h.svc.Process(c.Request.Context(), id, req, operatorFromClaims(c))
	if err != nil {
		Error(c, err)
		return
	}
	SuccessMessage(c, constants.MsgReviewRejected, dto.ToScoreReviewResponse(rev))
}

// Get 查看复核详情（学生本人或教师/管理员）。
func (h *ScoreReviewHandler) Get(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：id 参数非法"))
		return
	}
	rev, err := h.svc.GetForViewer(c.Request.Context(), id, operatorFromClaims(c))
	if err != nil {
		Error(c, err)
		return
	}
	Success(c, dto.ToScoreReviewResponse(rev))
}

// GetByRecord 按答卷查询复核（学生本人或教师/管理员；无复核返回 404）。
func (h *ScoreReviewHandler) GetByRecord(c *gin.Context) {
	recordID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：recordId 参数非法"))
		return
	}
	rev, err := h.svc.GetByRecordForViewer(c.Request.Context(), recordID, operatorFromClaims(c))
	if err != nil {
		Error(c, err)
		return
	}
	Success(c, dto.ToScoreReviewResponse(rev))
}

// ListMine 学生查询自己的复核申请。
func (h *ScoreReviewHandler) ListMine(c *gin.Context) {
	status := c.Query("status")
	page := util.GetPageParams(c, 20)
	list, total, err := h.svc.ListByStudent(c.Request.Context(), middleware.GetUserID(c), status, page.Page, page.PageSize)
	if err != nil {
		Error(c, err)
		return
	}
	items := make([]dto.ScoreReviewResponse, 0, len(list))
	for _, r := range list {
		items = append(items, dto.ToScoreReviewResponse(r))
	}
	PageResult(c, items, total, page.Page, page.PageSize)
}

// List 教师/管理员查询复核（可按 status / exam_id 过滤）。
func (h *ScoreReviewHandler) List(c *gin.Context) {
	filter := bson.M{}
	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}
	if examID := c.Query("exam_id"); examID != "" {
		oid, err := primitive.ObjectIDFromHex(examID)
		if err != nil {
			Error(c, util.NewAppError(constants.CodeBadRequest, "成绩复核模块：exam_id 参数非法（角色 teacher）"))
			return
		}
		filter["exam_id"] = oid
	}
	page := util.GetPageParams(c, 20)
	list, total, err := h.svc.ListForTeacher(c.Request.Context(), filter, page.Page, page.PageSize)
	if err != nil {
		Error(c, err)
		return
	}
	items := make([]dto.ScoreReviewResponse, 0, len(list))
	for _, r := range list {
		items = append(items, dto.ToScoreReviewResponse(r))
	}
	PageResult(c, items, total, page.Page, page.PageSize)
}
