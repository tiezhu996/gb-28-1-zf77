// 成绩复核状态（学生申请、教师处理、成绩页/批改页状态展示复用）。
import { create } from 'zustand';
import { scoreReviewApi, type ReviewDecisionInput } from '@/api/scoreReview';
import type { ScoreReview } from '@/types';

interface ScoreReviewState {
  mine: ScoreReview[];
  mineTotal: number;
  pending: ScoreReview[];
  pendingTotal: number;
  loading: boolean;
  fetchMine: (query?: { page?: number; page_size?: number }) => Promise<void>;
  fetchList: (query?: { status?: string; page?: number; page_size?: number }) => Promise<void>;
  create: (recordId: string, reason: string) => Promise<ScoreReview>;
  decide: (id: string, input: ReviewDecisionInput) => Promise<{ review: ScoreReview; record: unknown }>;
}

export const useScoreReviewStore = create<ScoreReviewState>((set) => ({
  mine: [],
  mineTotal: 0,
  pending: [],
  pendingTotal: 0,
  loading: false,
  fetchMine: async (query = { page: 1, page_size: 10 }) => {
    set({ loading: true });
    try {
      const res = await scoreReviewApi.mine(query);
      set({ mine: res.list, mineTotal: res.total });
    } finally {
      set({ loading: false });
    }
  },
  fetchList: async (query = { status: 'pending', page: 1, page_size: 10 }) => {
    set({ loading: true });
    try {
      const res = await scoreReviewApi.list(query);
      set({ pending: res.list, pendingTotal: res.total });
    } finally {
      set({ loading: false });
    }
  },
  create: (recordId, reason) => scoreReviewApi.create(recordId, reason),
  decide: (id, input) => scoreReviewApi.decide(id, input),
}));
