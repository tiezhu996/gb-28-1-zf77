package constants

// 统一文案集中定义：接口返回文案、日志文案、错误提示文案混合维护（屎山设计之一）。
// 注意：修改任何文案时，需要同步检查 handler 返回、service 日志与前端提示。
const (
	MsgOK                    = "ok"
	MsgUnauthorized          = "请先登录后再操作"
	MsgForbidden             = "当前角色无权执行该操作"
	MsgLoginSuccess          = "登录成功"
	MsgRegisterSuccess       = "注册成功"
	MsgLogoutSuccess         = "退出登录成功"
	MsgQuestionImportSuccess = "题目批量导入成功"
	MsgExamAutoGenerate      = "自动组卷完成"
	MsgExamPublishSuccess    = "试卷发布成功"
	MsgRecordSubmitSuccess   = "答卷提交成功"
	MsgRecordGradedSuccess   = "主观题批改完成"
	MsgWrongBookAdded        = "已加入错题本"
	MsgWrongBookResolved     = "已标记为已掌握"

	// 成绩复核模块文案（状态机 pending/approved/rejected，角色 student/teacher）
	MsgReviewCreated       = "成绩复核申请已提交，等待教师处理"
	MsgReviewApproved      = "复核已受理，成绩已按意见处理"
	MsgReviewRejected      = "复核已驳回"
	MsgReviewNotFound      = "成绩复核模块：id=%s 的复核申请不存在（角色 %s 无权查看或申请不存在）"
	MsgReviewExists        = "成绩复核模块：record_id=%s 的答卷已有复核申请，每份答卷仅允许一次（角色 student）"
	MsgReviewPendingExists = "成绩复核模块：record_id=%s 的答卷存在待处理复核申请，禁止重复提交（角色 student）"
	MsgReviewWindowClosed  = "成绩复核模块：record_id=%s 已超过批改完成后 48 小时复核窗口，申请被拒绝（角色 student）"
	MsgReviewNotGraded     = "成绩复核模块：record_id=%s 的答卷尚未批改完成（状态 %s），无法申请复核（角色 student）"
	MsgReviewStatusInvalid = "成绩复核模块：id=%s 的申请状态 %s 非法，仅待处理申请可受理/驳回（角色 %s）"
	MsgReviewOpinionEmpty  = "成绩复核模块：处理意见字段 opinion 不能为空（角色 %s）"
	MsgReviewScoreRange    = "成绩复核模块：更正总分字段 corrected_score=%v 超出 [0,%v] 范围（角色 teacher）"
	MsgReviewRecordLocked  = "成绩复核模块：record_id=%s 的答卷处于复核流程中，普通批改通道锁定（角色 teacher）"
	MsgReviewForbidden     = "成绩复核模块：角色 %s 无权操作该复核申请"
	MsgReviewReasonEmpty   = "成绩复核模块：申请理由字段 reason 不能为空（角色 student）"

	// 错误提示文案（与 error_codes.go 对应，但由 service/handler 手动拼接实体名、字段名、角色名）
	MsgValidationFailed    = "参数校验失败：字段 %s 不符合要求"
	MsgUserEmailExists     = "用户模块：邮箱字段 %s 已被注册"
	MsgUserNotFound        = "用户模块：id=%s 的用户不存在"
	MsgUserBadPassword     = "用户模块：密码字段不正确"
	MsgUserDisabled        = "用户模块：角色 %s 的用户已被禁用"
	MsgQuestionNotFound    = "题库模块：id=%s 的题目不存在"
	MsgQuestionTypeInvalid = "题库模块：题型字段 %s 非法"
	MsgExamNotFound        = "试卷模块：id=%s 的试卷不存在"
	MsgExamStatusInvalid   = "试卷模块：状态字段 %s 非法，无法从 %s 流转到 %s"
	MsgExamNotInWindow     = "试卷模块：考试时间窗口校验失败"
	MsgExamNoQuestions     = "试卷模块：题目列表为空，无法开始考试"
	MsgRecordNotFound      = "考试记录模块：id=%s 的记录不存在"
	MsgRecordStatusInvalid = "考试记录模块：状态字段 %s 非法，无法执行该操作"
	MsgRecordExpired       = "考试记录模块：考试时长已超时"
	MsgWrongBookExists     = "错题本模块：question_id=%s 已在错题本中"
)
