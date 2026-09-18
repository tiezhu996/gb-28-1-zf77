'use client';
import { Suspense, useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { recordApi } from '@/api/record';
import { scoreReviewApi } from '@/api/scoreReview';
import { StatusBadge } from '@/components/StatusBadge';
import { formatDateTime, reviewStatusColor, reviewStatusText } from '@/utils/format';
import { SCORE_REVIEW_STATUS, REVIEW_WINDOW_MS } from '@/constants';
import type { ExamRecord, ScoreReview } from '@/types';

function ApplyReview() {
  const router = useRouter();
  const params = useSearchParams();
  const recordId = params.get('recordId') ?? '';

  const [record, setRecord] = useState<ExamRecord | null>(null);
  const [review, setReview] = useState<ScoreReview | null>(null);
  const [reason, setReason] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    if (!recordId) return;
    try {
      const r = await recordApi.get(recordId);
      setRecord(r);
      setReview(r.review ?? null);
    } catch (err) {
      setError((err as Error).message);
    }
  }, [recordId]);

  useEffect(() => {
    load();
  }, [load]);

  if (!record) return <div className="p-10 text-center text-gray-400">{error || '加载中…'}</div>;

  const gradedAt = record.graded_at ? new Date(record.graded_at).getTime() : 0;
  const deadline = gradedAt + REVIEW_WINDOW_MS;
  const remainingMs = deadline - Date.now();
  const inWindow = remainingMs > 0;
  const hasPending = review?.status === SCORE_REVIEW_STATUS.PENDING;
  const finished = review && review.status !== SCORE_REVIEW_STATUS.PENDING;

  const submit = async () => {
    if (reason.trim().length < 2) {
      setError('请填写至少 2 个字符的复核理由');
      return;
    }
    setSubmitting(true);
    setError('');
    try {
      const rv = await scoreReviewApi.create(recordId, reason.trim());
      setReview(rv);
      alert('复核申请已提交，请等待教师处理');
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-xl font-bold text-gray-800">成绩复核申请</h1>

      <section className="rounded-xl border border-gray-200 bg-white p-5">
        <div className="flex items-center justify-between">
          <div>
            <p className="font-medium text-gray-800">{record.exam_title}</p>
            <p className="mt-1 text-sm text-gray-500">
              最终分：
              <span className={`ml-1 font-semibold ${record.score_adjusted ? 'text-green-600' : ''}`}>
                {record.effective_score}
              </span>
              {record.score_adjusted && (
                <span className="ml-1 text-xs text-gray-400 line-through">{record.final_score}</span>
              )}
              <span className="ml-3">
                及格状态：
                <StatusBadge text={record.passed ? '及格' : '不及格'} color={record.passed ? 'green' : 'red'} />
              </span>
            </p>
            <p className="mt-1 text-xs text-gray-400">批改完成时间：{formatDateTime(record.graded_at)}</p>
          </div>
          <Link href={`/records/review?recordId=${recordId}`} className="text-sm text-brand-600 hover:underline">查看答卷</Link>
        </div>
      </section>

      {/* 已有申请：只读展示处理留痕，杜绝重复提交 */}
      {review && (
        <section className="rounded-xl border border-gray-200 bg-white p-5">
          <div className="flex items-center justify-between">
            <h2 className="font-semibold text-gray-800">复核记录</h2>
            <StatusBadge text={reviewStatusText(review.status)} color={reviewStatusColor(review.status)} />
          </div>
          <p className="mt-3 text-sm"><span className="text-gray-500">申请理由：</span>{review.reason}</p>
          <p className="mt-1 text-xs text-gray-400">申请时间：{formatDateTime(review.created_at)}</p>
          {finished && (
            <div className="mt-3 rounded-lg bg-gray-50 p-3 text-sm">
              <p>
                <span className="text-gray-500">处理结果（{review.status === SCORE_REVIEW_STATUS.APPROVED ? '受理' : '驳回'}）：</span>
                教师 {review.handler_name} · {formatDateTime(review.handled_at)}
              </p>
              <p className="mt-1"><span className="text-gray-500">处理意见：</span>{review.comment}</p>
              {review.status === SCORE_REVIEW_STATUS.APPROVED && (
                <p className="mt-1 text-green-600">
                  分数 {review.original_score} → {review.corrected_score}；
                  及格状态：{review.corrected_passed ? '及格' : '不及格'}
                </p>
              )}
            </div>
          )}
          {hasPending && <p className="mt-3 text-sm text-orange-600">申请正在处理中，同一答卷不可重复提交。</p>}
        </section>
      )}

      {/* 仅窗口期内且无申请时可填写 */}
      {!review && inWindow && record.status === 'graded' && (
        <section className="rounded-xl border border-gray-200 bg-white p-5">
          <h2 className="font-semibold text-gray-800">填写复核理由</h2>
          <p className="mt-1 text-xs text-gray-400">
            距申请截止还剩 {Math.max(0, Math.floor(remainingMs / 3600000))} 小时；提交后教师将在处理时填写意见并全程留痕。
          </p>
          <textarea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            maxLength={500}
            rows={5}
            placeholder="请说明申请复核的理由，例如：第 3 题简答题答到得分点但未给分……"
            className="mt-3 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
          />
          <div className="mt-2 text-right text-xs text-gray-400">{reason.length}/500</div>
          {error && <p className="mt-2 text-sm text-red-600">{error}</p>}
          <div className="mt-3 flex justify-end gap-2">
            <button onClick={() => router.back()} className="rounded-lg border border-gray-300 px-4 py-2 text-sm hover:bg-gray-50">取消</button>
            <button onClick={submit} disabled={submitting}
              className="rounded-lg bg-orange-600 px-5 py-2 text-sm text-white hover:bg-orange-700 disabled:opacity-60">
              {submitting ? '提交中…' : '提交复核申请'}
            </button>
          </div>
        </section>
      )}

      {!review && (!inWindow || record.status !== 'graded') && (
        <section className="rounded-xl border border-red-200 bg-red-50 p-5 text-sm text-red-700">
          {record.status !== 'graded'
            ? '答卷尚未批改完成，批改完成后才可申请复核。'
            : '已超过批改完成后的 48 小时复核窗口期，无法发起复核。'}
        </section>
      )}
    </div>
  );
}

export default function ApplyReviewPage() {
  return (
    <Suspense fallback={<div className="p-10 text-center text-gray-400">加载中…</div>}>
      <ApplyReview />
    </Suspense>
  );
}
