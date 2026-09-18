package router

import (
	"github.com/gin-gonic/gin"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/handler"
	"github.com/onlineexam/onlineexam/internal/middleware"
)

// RegisterScoreReviewRoutes 成绩复核模块路由。
// 学生：窗口内对本人答卷发起复核、查询本人申请、按答卷查询复核状态。
// 教师/管理员：查询待处理列表、驳回/受理并更正、按答卷查询复核状态。
func RegisterScoreReviewRoutes(g *gin.RouterGroup, h *handler.ScoreReviewHandler) {
	student := g.Group("/score-reviews", middleware.RequireRoles(constants.RoleStudent))
	{
		student.POST("/records/:id", h.Create) // 按答卷发起复核
		student.GET("/mine", h.ListMine)
	}

	teacher := g.Group("/score-reviews", middleware.RequireRoles(constants.RoleTeacher, constants.RoleAdmin))
	{
		teacher.GET("", h.List)
		teacher.GET("/:id", h.Get)
		teacher.POST("/:id/decision", h.Decide) // 驳回 / 受理并更正
	}

	// 按答卷查询复核状态：学生与教师都可访问（归属校验在 handler 内）
	g.GET("/exam-records/:id/review",
		middleware.RequireRoles(constants.RoleStudent, constants.RoleTeacher, constants.RoleAdmin),
		h.GetByRecord)
}
