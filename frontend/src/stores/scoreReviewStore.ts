// 成绩复核状态（学生申请、教师处理共用）。
import { create } from 'zustand';
import { scoreReviewApi } from '@/api/scoreReview';
import type { ScoreReview } from '@/types';

interface ScoreReviewState {
  list: ScoreReview[];
  total: number;
  loading: boolean;
  fetchMine: (query: { status?: string; page?: number; page_size?: number }) => Promise<void>;
  fetchAll: (query: { status?: string; exam_id?: string; page?: number; page_size?: number }) => Promise<void>;
  getByRecord: (recordId: string) => Promise<ScoreReview | null>;
  create: (recordId: string, reason: string) => Promise<ScoreReview>;
  approve: (id: string, opinion: string, correctedScore?: number | null) => Promise<ScoreReview>;
  reject: (id: string, opinion: string) => Promise<ScoreReview>;
}

export const useScoreReviewStore = create<ScoreReviewState>((set) => ({
  list: [],
  total: 0,
  loading: false,
  fetchMine: async (query) => {
    set({ loading: true });
    try {
      const res = await scoreReviewApi.mine(query);
      set({ list: res.list, total: res.total });
    } finally {
      set({ loading: false });
    }
  },
  fetchAll: async (query) => {
    set({ loading: true });
    try {
      const res = await scoreReviewApi.list(query);
      set({ list: res.list, total: res.total });
    } finally {
      set({ loading: false });
    }
  },
  getByRecord: async (recordId) => {
    try {
      return await scoreReviewApi.getByRecord(recordId);
    } catch {
      return null;
    }
  },
  create: (recordId, reason) => scoreReviewApi.create(recordId, reason),
  approve: (id, opinion, correctedScore) => scoreReviewApi.approve(id, opinion, correctedScore),
  reject: (id, opinion) => scoreReviewApi.reject(id, opinion),
}));
