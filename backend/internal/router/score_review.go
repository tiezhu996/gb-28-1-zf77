package router

import (
	"github.com/gin-gonic/gin"

	"github.com/onlineexam/onlineexam/internal/constants"
	"github.com/onlineexam/onlineexam/internal/handler"
	"github.com/onlineexam/onlineexam/internal/middleware"
)

// RegisterScoreReviewRoutes 成绩复核模块路由。
// 学生：对本人答卷发起复核、查看自己的复核单；
// 教师/管理员：查询全部复核、受理/驳回。
// 越权访问（学生查他人单/操作他人单）在 service 层二次拦截。
func RegisterScoreReviewRoutes(g *gin.RouterGroup, h *handler.ScoreReviewHandler) {
	// 学生动作
	student := g.Group("/exam-records", middleware.RequireRoles(constants.RoleStudent))
	{
		student.POST("/:id/reviews", h.Create)
	}
	studentReviews := g.Group("/score-reviews", middleware.RequireRoles(constants.RoleStudent))
	{
		studentReviews.GET("/mine", h.ListMine)
	}

	// 教师/管理员动作
	teacher := g.Group("/score-reviews", middleware.RequireRoles(constants.RoleTeacher, constants.RoleAdmin))
	{
		teacher.GET("", h.List)
		teacher.POST("/:id/approve", h.Approve)
		teacher.POST("/:id/reject", h.Reject)
	}

	// 师生共用的详情查询（service 层按角色与归属校验）
	g.GET("/score-reviews/:id", middleware.RequireRoles(constants.RoleStudent, constants.RoleTeacher, constants.RoleAdmin), h.Get)
	g.GET("/exam-records/:id/review", middleware.RequireRoles(constants.RoleStudent, constants.RoleTeacher, constants.RoleAdmin), h.GetByRecord)
}
