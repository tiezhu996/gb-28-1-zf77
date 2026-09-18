import { request, buildQuery } from '@/utils/request';
import type { PageResult, ScoreReview } from '@/types';

export interface CreateReviewInput {
  reason: string;
}

export interface ReviewDecisionInput {
  action: 'approve' | 'reject';
  comment: string;
  corrected_score?: number;
  passed_override?: boolean | null;
}

export const scoreReviewApi = {
  // 学生对指定答卷发起复核
  create(recordId: string, reason: string) {
    return request<ScoreReview>(`/score-reviews/records/${recordId}`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    });
  },
  // 学生查询本人复核申请
  mine(query: { page?: number; page_size?: number }) {
    return request<PageResult<ScoreReview>>(`/score-reviews/mine${buildQuery({ ...query })}`);
  },
  // 教师查询复核列表（默认待处理）
  list(query: { status?: string; page?: number; page_size?: number }) {
    return request<PageResult<ScoreReview>>(`/score-reviews${buildQuery({ ...query })}`);
  },
  // 查询单条复核
  get(id: string) {
    return request<ScoreReview>(`/score-reviews/${id}`);
  },
  // 按答卷查询复核状态（学生成绩页、教师批改页复用）
  getByRecord(recordId: string) {
    return request<ScoreReview>(`/exam-records/${recordId}/review`);
  },
  // 教师驳回 / 受理并更正
  decide(id: string, input: ReviewDecisionInput) {
    return request<{ review: ScoreReview; record: unknown }>(`/score-reviews/${id}/decision`, {
      method: 'POST',
      body: JSON.stringify(input),
    });
  },
};
