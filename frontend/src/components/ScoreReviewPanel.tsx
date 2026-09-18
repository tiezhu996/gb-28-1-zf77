'use client';
// 成绩复核面板（答卷详情页复用）：学生展示申请入口与状态、教师展示受理/驳回操作、全程留痕时间线。
import { useState } from 'react';
import { useAuth } from '@/hooks/useAuth';
import { scoreReviewApi } from '@/api/scoreReview';
import { StatusBadge } from '@/components/StatusBadge';
import { formatDateTime, reviewStatusColor, reviewStatusText } from '@/utils/format';
import { REVIEW_STATUS } from '@/constants';
import type { ExamRecord, ScoreReview } from '@/types';

export function ScoreReviewPanel({
  record,
  review,
  onChanged,
}: {
  record: ExamRecord;
  review: ScoreReview | null;
  onChanged: () => void;
}) {
  const { isTeacher, isAdmin, isStudent } = useAuth();
  const canHandle = isTeacher || isAdmin;

  const [reason, setReason] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [opinion, setOpinion] = useState('');
  const [correctMode, setCorrectMode] = useState(false);
  const [newScore, setNewScore] = useState<number | ''>('');
  const [processing, setProcessing] = useState(false);

  const passedText = (passed: boolean) => (passed ? '及格' : '不及格');

  const submitReview = async () => {
    if (reason.trim().length < 2) {
      alert('请填写复核理由（至少 2 个字）');
      return;
    }
    setSubmitting(true);
    try {
      await scoreReviewApi.create(record.id, reason.trim());
      alert('复核申请已提交，请等待教师处理');
      setReason('');
      onChanged();
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleApprove = async () => {
    if (opinion.trim().length < 2) {
      alert('请填写受理意见（必填）');
      return;
    }
    const score = correctMode && newScore !== '' ? Number(newScore) : null;
    if (correctMode && (newScore === '' || Number.isNaN(Number(newScore)))) {
      alert('请输入更正后的总分');
      return;
    }
    if (!confirm(correctMode && score !== null ? `确认受理并将总分更正为 ${score} 分？` : '确认受理（维持原分）？')) return;
    setProcessing(true);
    try {
      await scoreReviewApi.approve(review!.id, opinion.trim(), score);
      alert('已受理');
      setOpinion('');
      setCorrectMode(false);
      setNewScore('');
      onChanged();
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setProcessing(false);
    }
  };

  const handleReject = async () => {
    if (opinion.trim().length < 2) {
      alert('请填写驳回意见（必填）');
      return;
    }
    if (!confirm('确认驳回该复核申请？原成绩保持不变。')) return;
    setProcessing(true);
    try {
      await scoreReviewApi.reject(review!.id, opinion.trim());
      alert('已驳回');
      setOpinion('');
      onChanged();
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setProcessing(false);
    }
  };

  return (
    <section className="rounded-xl border border-gray-200 bg-white p-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="font-semibold text-gray-800">成绩复核</h2>
        <StatusBadge
          text={review ? reviewStatusText(review.status) : reviewStatusText(REVIEW_STATUS.NONE)}
          color={review ? reviewStatusColor(review.status) : 'gray'}
        />
      </div>

      {/* 最终成绩与及格状态（复核更正后展示最终分） */}
      <div className="mt-3 flex flex-wrap gap-4 rounded-lg bg-gray-50 p-3 text-sm">
        <span>
          最终总分：<span className={`font-bold ${record.score_corrected ? 'text-orange-600' : 'text-brand-600'}`}>{record.final_score ?? '-'}</span>
          {record.score_corrected && <span className="ml-1 text-xs text-orange-500">（复核已更正）</span>}
        </span>
        <span>
          及格状态：
          <span className={record.is_passed ? 'text-green-600 font-medium' : 'text-red-600 font-medium'}>
            {record.pass_score > 0 ? passedText(record.is_passed) : '未设及格线'}
          </span>
        </span>
        {record.graded_at && (
          <span className="text-xs text-gray-400">批改完成：{formatDateTime(record.graded_at)}</span>
        )}
      </div>

      {/* 无复核：学生在 48h 窗口内可发起一次 */}
      {!review && isStudent && (
        <div className="mt-4">
          {record.review_window_open ? (
            <>
              <p className="text-xs text-gray-500">
                可在批改完成后 48 小时内发起一次成绩复核，截止 {formatDateTime(record.review_deadline)}。
              </p>
              <textarea
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                maxLength={500}
                rows={3}
                placeholder="请说明复核理由（必填，2-500 字）"
                className="mt-2 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
              />
              <div className="mt-2 flex justify-end">
                <button
                  onClick={submitReview}
                  disabled={submitting}
                  className="rounded-lg bg-orange-600 px-4 py-1.5 text-sm text-white hover:bg-orange-700 disabled:opacity-60"
                >
                  {submitting ? '提交中…' : '发起复核'}
                </button>
              </div>
            </>
          ) : (
            <p className="mt-2 text-xs text-gray-400">
              {record.status === 'graded'
                ? '复核窗口已关闭（批改完成超过 48 小时）或已发起过复核。'
                : '答卷批改完成后才可发起成绩复核。'}
            </p>
          )}
        </div>
      )}

      {/* 复核详情 */}
      {review && (
        <div className="mt-4 space-y-3">
          <div className="rounded-lg border border-orange-100 bg-orange-50 p-3 text-sm">
            <p className="text-xs text-gray-500">学生申请理由 · {formatDateTime(review.created_at)}</p>
            <p className="mt-1 text-gray-800">{review.reason}</p>
            <p className="mt-1 text-xs text-gray-500">
              申请时成绩：{review.original_score} 分（{passedText(review.original_passed)}）
            </p>
          </div>

          {review.status !== REVIEW_STATUS.PENDING && review.teacher_opinion && (
            <div className="rounded-lg border border-blue-100 bg-blue-50 p-3 text-sm">
              <p className="text-xs text-gray-500">
                教师{review.status === REVIEW_STATUS.APPROVED ? '受理' : '驳回'}意见 · {review.teacher_name} · {formatDateTime(review.processed_at)}
              </p>
              <p className="mt-1 text-gray-800">{review.teacher_opinion}</p>
              {review.status === REVIEW_STATUS.APPROVED && (
                <p className="mt-1 text-xs">
                  更正结果：
                  <span className={review.score_corrected ? 'text-orange-600 font-medium' : 'text-gray-500'}>
                    {review.score_corrected
                      ? `${review.original_score} → ${review.corrected_score} 分（${passedText(review.corrected_passed)}）`
                      : `维持原分 ${review.original_score} 分`}
                  </span>
                </p>
              )}
            </div>
          )}

          {/* 教师处理区：仅待处理时可操作 */}
          {canHandle && review.status === REVIEW_STATUS.PENDING && (
            <div className="rounded-lg border border-gray-200 p-3">
              <p className="text-sm font-medium text-gray-700">处理复核（意见必填）</p>
              <textarea
                value={opinion}
                onChange={(e) => setOpinion(e.target.value)}
                maxLength={500}
                rows={2}
                placeholder="受理/驳回意见（必填，2-500 字，全程留痕）"
                className="mt-2 w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
              />
              <label className="mt-2 flex items-center gap-2 text-xs text-gray-600">
                <input type="checkbox" checked={correctMode} onChange={(e) => setCorrectMode(e.target.checked)} />
                受理并更正总分（不勾选表示受理但维持原分）
              </label>
              {correctMode && (
                <input
                  type="number"
                  min={0}
                  step={0.5}
                  value={newScore}
                  onChange={(e) => setNewScore(e.target.value === '' ? '' : Number(e.target.value))}
                  placeholder="更正后的总分"
                  className="mt-2 w-40 rounded-lg border border-gray-300 px-3 py-1.5 text-sm"
                />
              )}
              <div className="mt-3 flex justify-end gap-2">
                <button
                  onClick={handleReject}
                  disabled={processing}
                  className="rounded-lg border border-red-300 px-4 py-1.5 text-sm text-red-600 hover:bg-red-50 disabled:opacity-60"
                >
                  驳回
                </button>
                <button
                  onClick={handleApprove}
                  disabled={processing}
                  className="rounded-lg bg-brand-600 px-4 py-1.5 text-sm text-white hover:bg-brand-700 disabled:opacity-60"
                >
                  {processing ? '处理中…' : '受理'}
                </button>
              </div>
            </div>
          )}

          {/* 留痕时间线 */}
          <div className="border-t border-gray-100 pt-3">
            <p className="text-xs font-medium text-gray-500">处理留痕</p>
            <ol className="mt-2 space-y-2">
              {review.history.map((h, i) => (
                <li key={i} className="flex items-start gap-2 text-xs">
                  <span className="mt-0.5 inline-block h-2 w-2 shrink-0 rounded-full bg-brand-400" />
                  <div>
                    <p className="text-gray-700">
                      {h.action_text} · {h.operator_name}（{h.operator_role === 'teacher' ? '教师' : h.operator_role === 'admin' ? '管理员' : '学生'}） · {formatDateTime(h.occurred_at)}
                    </p>
                    <p className="mt-0.5 text-gray-500">{h.opinion}</p>
                  </div>
                </li>
              ))}
            </ol>
          </div>
        </div>
      )}
    </section>
  );
}
