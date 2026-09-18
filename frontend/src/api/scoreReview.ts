import { request, buildQuery } from '@/utils/request';
import type { PageResult, ScoreReview } from '@/types';

// 成绩复核闭环 API（学生申请、教师受理/驳回、全程留痕）
export const scoreReviewApi = {
  // 学生对本人答卷发起一次复核（批改完成 48h 内）
  create(recordId: string, reason: string) {
    return request<ScoreReview>(`/exam-records/${recordId}/reviews`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    });
  },
  // 按答卷查询复核（学生本人 / 教师管理员）
  getByRecord(recordId: string) {
    return request<ScoreReview>(`/exam-records/${recordId}/review`);
  },
  get(id: string) {
    return request<ScoreReview>(`/score-reviews/${id}`);
  },
  // 学生查询自己的复核申请
  mine(query: { status?: string; page?: number; page_size?: number }) {
    return request<PageResult<ScoreReview>>(`/score-reviews/mine${buildQuery({ ...query })}`);
  },
  // 教师/管理员查询全部复核
  list(query: { status?: string; exam_id?: string; page?: number; page_size?: number }) {
    return request<PageResult<ScoreReview>>(`/score-reviews${buildQuery({ ...query })}`);
  },
  // 教师受理并更正总分（不传 corrected_score 表示受理但维持原分）
  approve(id: string, opinion: string, correctedScore?: number | null) {
    return request<ScoreReview>(`/score-reviews/${id}/approve`, {
      method: 'POST',
      body: JSON.stringify({ opinion, corrected_score: correctedScore ?? null }),
    });
  },
  // 教师驳回（必须填写意见，不得改分）
  reject(id: string, opinion: string) {
    return request<ScoreReview>(`/score-reviews/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ opinion }),
    });
  },
};
