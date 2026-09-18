'use client';
import { Suspense, useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { recordApi } from '@/api/record';
import { wrongBookApi } from '@/api/wrongBook';
import { QuestionTypeBadge, StatusBadge } from '@/components/StatusBadge';
import { answerResultText, formatDateTime, recordStatusColor, recordStatusText, reviewStatusColor, reviewStatusText } from '@/utils/format';
import { ANSWER_RESULT, SCORE_REVIEW_STATUS, REVIEW_WINDOW_MS } from '@/constants';
import type { ExamRecord } from '@/types';

function Review() {
  const params = useSearchParams();
  const recordId = params.get('recordId') ?? '';
  const { isTeacher, isAdmin, isStudent } = useAuth();
  const canGrade = isTeacher || isAdmin;

  const [record, setRecord] = useState<ExamRecord | null>(null);
  const [scores, setScores] = useState<Record<string, number>>({});
  const [comments, setComments] = useState<Record<string, string>>({});
  const [grading, setGrading] = useState(false);
  const [adding, setAdding] = useState(false);

  const load = useCallback(async () => {
    if (!recordId) return;
    const r = await recordApi.get(recordId);
    setRecord(r);
    const sc: Record<string, number> = {};
    const cm: Record<string, string> = {};
    r.questions.forEach((q) => {
      if (q.subjective_score) sc[q.question_id] = q.subjective_score;
      if (q.comment) cm[q.question_id] = q.comment;
    });
    setScores(sc);
    setComments(cm);
  }, [recordId]);

  useEffect(() => {
    load();
  }, [load]);

  if (!record) return <div className="p-10 text-center text-gray-400">加载中…</div>;

  const inReviewWindow = !!record.graded_at && Date.now() - new Date(record.graded_at).getTime() <= REVIEW_WINDOW_MS;

  const resultColor = (r: string) => {
    switch (r) {
      case ANSWER_RESULT.CORRECT: return 'green';
      case ANSWER_RESULT.WRONG: return 'red';
      case ANSWER_RESULT.PARTIAL: return 'orange';
      default: return 'gray';
    }
  };

  const onGrade = async () => {
    setGrading(true);
    try {
      const grades = record.questions
        .filter((q) => q.type === 'fill' || q.type === 'short')
        .map((q) => ({ question_id: q.question_id, score: scores[q.question_id] ?? 0, comment: comments[q.question_id] ?? '' }));
      await recordApi.grade(record.id, grades);
      alert('批改完成');
      load();
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setGrading(false);
    }
  };

  const onAddWrong = async (q: { question_id: string }) => {
    setAdding(true);
    try {
      await wrongBookApi.add({ question_id: q.question_id, exam_id: record.exam_id, exam_record_id: record.id });
      alert('已加入错题本');
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setAdding(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-bold text-gray-800">{record.exam_title} · 答卷详情</h1>
          <p className="mt-1 text-sm text-gray-500">
            学生 {record.student_name} · 客观题 {record.objective_score} 分 ·{' '}
            最终{' '}
            <span className={`font-semibold ${record.score_adjusted ? 'text-green-600' : ''}`}>
              {record.status === 'graded' ? record.effective_score : '-'}
            </span>
            {record.score_adjusted && (
              <span className="ml-1 text-xs text-gray-400 line-through">{record.final_score}</span>
            )}
            {' '}分 · 切屏 {record.cheat_count} 次
            {record.status === 'graded' && (
              <span className="ml-2">
                <StatusBadge text={record.passed ? '及格' : '不及格'} color={record.passed ? 'green' : 'red'} />
              </span>
            )}
          </p>
          <p className="text-xs text-gray-400">
            开始 {formatDateTime(record.started_at)}
            {record.graded_at ? ` · 批改完成 ${formatDateTime(record.graded_at)}` : ''}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {record.review && (
            <StatusBadge text={reviewStatusText(record.review.status)} color={reviewStatusColor(record.review.status)} />
          )}
          <StatusBadge text={recordStatusText(record.status)} color={recordStatusColor(record.status)} />
        </div>
      </div>

      {/* 复核状态条：学生申请/查看，教师前往处理；更正后的最终分数在上方展示 */}
      {record.status === 'graded' && (
        <section className="rounded-xl border border-gray-200 bg-white p-4">
          {record.review ? (
            <div className="space-y-2 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <StatusBadge text={reviewStatusText(record.review.status)} color={reviewStatusColor(record.review.status)} />
                <span className="text-gray-500">申请理由：</span>
                <span>{record.review.reason}</span>
                <span className="text-xs text-gray-400">{formatDateTime(record.review.created_at)}</span>
              </div>
              {record.review.status === SCORE_REVIEW_STATUS.PENDING ? (
                <p className="text-xs text-orange-600">
                  {canGrade
                    ? '该复核正在等待处理，请在「成绩复核处理」页驳回或受理。'
                    : '复核申请正在处理中，请耐心等待教师意见。'}
                </p>
              ) : (
                <div className="rounded-lg bg-gray-50 p-3">
                  <p className="text-gray-600">
                    {record.review.handler_name} 于 {formatDateTime(record.review.handled_at)} 的处理意见：
                    {record.review.comment}
                  </p>
                  {record.review.status === SCORE_REVIEW_STATUS.APPROVED && (
                    <p className="mt-1 text-green-600">
                      成绩已更正：{record.review.original_score} → {record.review.corrected_score} 分；
                      最终及格状态：{record.review.corrected_passed ? '及格' : '不及格'}
                    </p>
                  )}
                </div>
              )}
              {canGrade && record.review.status === SCORE_REVIEW_STATUS.PENDING && (
                <Link href="/reviews" className="inline-block text-sm text-brand-600 hover:underline">前往处理复核 →</Link>
              )}
            </div>
          ) : (
            <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
              <span className="text-gray-500">
                {isStudent
                  ? (inReviewWindow ? '如对成绩有异议，可在批改完成后 48 小时内申请一次复核。' : '已超过批改完成后的 48 小时复核窗口期。')
                  : '该答卷暂无复核申请。'}
              </span>
              {isStudent && inReviewWindow && (
                <Link href={`/reviews/apply?recordId=${record.id}`} className="rounded-lg bg-orange-600 px-4 py-1.5 text-xs text-white hover:bg-orange-700">申请成绩复核</Link>
              )}
            </div>
          )}
          {record.adjustment && (
            <p className="mt-2 text-xs text-gray-400">
              原始批改明细保留不变，本页题目得分仍为批改时给分；上方“最终分/及格状态”为复核更正后的结果。
            </p>
          )}
        </section>
      )}

      {record.questions.map((q, i) => (
        <div key={q.question_id} className="rounded-xl border border-gray-200 bg-white p-5">
          <div className="flex items-center justify-between">
            <span className="text-sm text-gray-400">第 {i + 1} 题</span>
            <div className="flex items-center gap-2">
              <QuestionTypeBadge type={q.type} />
              <StatusBadge text={answerResultText(q.result)} color={resultColor(q.result)} />
            </div>
          </div>
          <p className="mt-2 font-medium text-gray-800">{q.content}</p>
          <p className="mt-1 text-xs text-gray-400">{q.score} 分 · 本题得分 {q.got_score || 0}</p>

          {(q.type === 'single' || q.type === 'multiple' || q.type === 'judge') && (
            <div className="mt-3 space-y-1">
              {q.type === 'judge'
                ? ['true', 'false'].map((v) => (
                    <p key={v} className="text-sm">
                      {v === 'true' ? '正确' : '错误'}
                      {q.correct_answer === v && <span className="ml-2 text-green-600">✓ 正确答案</span>}
                      {q.user_answer === v && <span className={`ml-2 ${q.user_answer === q.correct_answer ? 'text-green-600' : 'text-red-600'}`}>我的答案</span>}
                    </p>
                  ))
                : (q.options ?? []).map((o) => (
                    <p key={o.key} className="text-sm">
                      {o.key}. {o.text}
                      {q.correct_answer.split(',').includes(o.key) && <span className="ml-2 text-green-600">✓ 正确答案</span>}
                      {(q.user_answer ?? '').split(',').includes(o.key) && (
                        <span className={`ml-2 ${(q.correct_answer ?? '').split(',').includes(o.key) ? 'text-green-600' : 'text-red-600'}`}>我的答案</span>
                      )}
                    </p>
                  ))}
            </div>
          )}

          {q.type === 'fill' || q.type === 'short' ? (
            <div className="mt-3 rounded-lg bg-gray-50 p-3 text-sm">
              <p><span className="text-gray-500">我的答案：</span>{q.user_answer || '（未作答）'}</p>
              <p className="mt-1"><span className="text-gray-500">参考答案：</span>{q.correct_answer}</p>
              {canGrade && (
                <div className="mt-3 grid gap-2 sm:grid-cols-2">
                  <input type="number" min={0} max={q.score} step={0.5} value={scores[q.question_id] ?? ''}
                    placeholder="给分"
                    onChange={(e) => setScores({ ...scores, [q.question_id]: Number(e.target.value) })}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm" />
                  <input value={comments[q.question_id] ?? ''} placeholder="评语（可选）"
                    onChange={(e) => setComments({ ...comments, [q.question_id]: e.target.value })}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm" />
                </div>
              )}
            </div>
          ) : (
            <div className="mt-2 text-sm">
              <p><span className="text-gray-500">我的答案：</span>{q.user_answer || '（未作答）'}</p>
            </div>
          )}

          <div className="mt-3 flex items-center justify-between border-t border-gray-100 pt-3">
            <p className="text-xs text-gray-400">本题得分 {q.got_score || 0}/{q.score}</p>
            {isStudent && q.result === ANSWER_RESULT.WRONG && (
              <button onClick={() => onAddWrong(q)} disabled={adding}
                className="rounded-lg border border-amber-500 px-3 py-1 text-xs text-amber-600 hover:bg-amber-50 disabled:opacity-60">
                加入错题本
              </button>
            )}
          </div>
        </div>
      ))}

      {canGrade && record.status !== 'graded' && (
        <div className="flex justify-end">
          <button onClick={onGrade} disabled={grading}
            className="rounded-lg bg-brand-600 px-6 py-2 text-sm text-white hover:bg-brand-700 disabled:opacity-60">
            {grading ? '批改中…' : '保存批改'}
          </button>
        </div>
      )}
    </div>
  );
}

export default function ReviewPage() {
  return (
    <Suspense fallback={<div className="p-10 text-center text-gray-400">加载中…</div>}>
      <Review />
    </Suspense>
  );
}
