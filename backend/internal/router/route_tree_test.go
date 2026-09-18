package router

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/onlineexam/onlineexam/internal/handler"
)

// TestRouteTreeNoConflict 验证成绩复核路由与考试记录路由在同一棵
// httprouter 树上静态段（mine/records）与参数段（:id）共存不会 panic。
func TestRouteTreeNoConflict(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("路由注册冲突 panic: %v", r)
		}
	}()
	gin.SetMode(gin.TestMode)
	e := gin.New()
	g := e.Group("/api/v1")
	RegisterExamRecordRoutes(g, (*handler.ExamRecordHandler)(nil))
	RegisterScoreReviewRoutes(g, (*handler.ScoreReviewHandler)(nil))
}
