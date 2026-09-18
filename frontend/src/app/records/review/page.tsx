'use client';
import { Suspense, useCallback, useEffect, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { recordApi } from '@/api/record';
import { wrongBookApi } from '@/api/wrongBook';
import { scoreReviewApi } from '@/api/scoreReview';
import { QuestionTypeBadge, StatusBadge } from '@/components/StatusBadge';
import { ScoreReviewPanel } from '@/components/ScoreReviewPanel';
import { answerResultText, formatDateTime, recordStatusColor, recordStatusText } from '@/utils/format';
import { ANSWER_RESULT } from '@/constants';
import type { ExamRecord, ScoreReview } from '@/types';

function Review() {
  const params = useSearchParams();
  const recordId = params.get('recordId') ?? '';
  const { isTeacher, isAdmin, isStudent } = useAuth();
  const canGrade = isTeacher || isAdmin;

  const [record, setRecord] = useState<ExamRecord | null>(null);
  const [scoreReview, setScoreReview] = useState<ScoreReview | null>(null);
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
    // 复核状态：无复核时接口返回 404，置 null
    try {
      const rv = await scoreReviewApi.getByRecord(recordId);
      setScoreReview(rv);
    } catch {
      setScoreReview(null);
    }
  }, [recordId]);

  useEffect(() => {
    load();
  }, [load]);

  if (!record) return <div className="p-10 text-center text-gray-400">加载中…</div>;

  const resultColor = (r: string) => {
    switch (r) {
      case ANSWER_RESULT.CORRECT: return 'green';
      case ANSWER_RESULT.WRONG: return 'red';
      case ANSWER_RESULT.PARTIAL: return 'orange';
      default: return 'gray';
    }
  };

  // 复核存在时普通批改通道锁定（后端同样强制），防止改分绕过复核流程
  const reviewExists = !!scoreReview || record.review_status !== 'none';
  const gradeLocked = reviewExists;

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
            学生 {record.student_name} · 客观题 {record.objective_score} 分 · 最终{' '}
            <span className={record.score_corrected ? 'font-semibold text-orange-600' : 'font-semibold'}>
              {record.final_score || '-'}
            </span>
            {record.score_corrected && <span className="ml-1 text-xs text-orange-500">（复核已更正）</span>}
            {' '}分
            {record.pass_score > 0 && (
              <span className={`ml-2 font-medium ${record.is_passed ? 'text-green-600' : 'text-red-600'}`}>
                {record.is_passed ? '及格' : '不及格'}
              </span>
            )}
            {' '}· 切屏 {record.cheat_count} 次
          </p>
          <p className="text-xs text-gray-400">
            开始 {formatDateTime(record.started_at)}
            {record.graded_at ? ` · 批改完成 ${formatDateTime(record.graded_at)}` : ''}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <StatusBadge text={recordStatusText(record.status)} color={recordStatusColor(record.status)} />
          {record.review_status && record.review_status !== 'none' && (
            <StatusBadge
              text={record.review_status === 'pending' ? '复核中' : record.review_status === 'approved' ? '复核已受理' : '复核已驳回'}
              color={record.review_status === 'pending' ? 'orange' : record.review_status === 'approved' ? 'green' : 'red'}
            />
          )}
        </div>
      </div>

      {/* 成绩复核闭环面板：学生申请、教师受理/驳回、全程留痕 */}
      <ScoreReviewPanel record={record} review={scoreReview} onChanged={load} />

      {gradeLocked && canGrade && (
        <div className="rounded-xl border border-orange-200 bg-orange-50 p-4 text-sm text-orange-700">
          该答卷存在成绩复核申请，普通批改通道已锁定。请在上方“成绩复核”面板中受理（可更正总分）或驳回；重复批改不会改变成绩。
        </div>
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
                    disabled={gradeLocked}
                    onChange={(e) => setScores({ ...scores, [q.question_id]: Number(e.target.value) })}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm disabled:cursor-not-allowed disabled:bg-gray-100" />
                  <input value={comments[q.question_id] ?? ''} placeholder="评语（可选）"
                    disabled={gradeLocked}
                    onChange={(e) => setComments({ ...comments, [q.question_id]: e.target.value })}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm disabled:cursor-not-allowed disabled:bg-gray-100" />
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

      {canGrade && record.status !== 'graded' && !gradeLocked && (
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
