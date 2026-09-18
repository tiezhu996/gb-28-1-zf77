'use client';
import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { scoreReviewApi } from '@/api/scoreReview';
import { recordApi } from '@/api/record';
import { DataTable, type Column } from '@/components/DataTable';
import { Pagination } from '@/components/Pagination';
import { StatusBadge } from '@/components/StatusBadge';
import { Modal } from '@/components/Modal';
import { formatDateTime, reviewStatusColor, reviewStatusText } from '@/utils/format';
import { SCORE_REVIEW_ACTION, SCORE_REVIEW_STATUS } from '@/constants';
import type { ExamRecord, ScoreReview } from '@/types';
import type { ReviewDecisionInput } from '@/api/scoreReview';

type Tab = 'pending' | 'approved' | 'rejected' | '';

export default function TeacherReviewsPage() {
  const [list, setList] = useState<ScoreReview[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [tab, setTab] = useState<Tab>(SCORE_REVIEW_STATUS.PENDING as Tab);

  const [target, setTarget] = useState<ScoreReview | null>(null);
  const [record, setRecord] = useState<ExamRecord | null>(null);
  const [action, setAction] = useState<string>(SCORE_REVIEW_ACTION.REJECT);
  const [comment, setComment] = useState('');
  const [correctedScore, setCorrectedScore] = useState<string>('');
  const [passedOverride, setPassedOverride] = useState<string>(''); // '' 自动 / true / false
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const res = await scoreReviewApi.list({ status: tab || undefined, page, page_size: 10 });
      setList(res.list);
      setTotal(res.total);
    } finally {
      setLoading(false);
    }
  }, [tab, page]);

  useEffect(() => {
    reload();
  }, [reload]);

  const openDialog = async (rv: ScoreReview, act: string) => {
    setError('');
    setAction(act);
    setComment('');
    setPassedOverride('');
    try {
      const rec = await recordApi.get(rv.record_id);
      setRecord(rec);
      setCorrectedScore(String(rec.effective_score ?? rec.final_score ?? 0));
      setTarget(rv);
    } catch (err) {
      alert((err as Error).message);
    }
  };

  const closeDialog = () => {
    setTarget(null);
    setRecord(null);
  };

  const submit = async () => {
    if (!target) return;
    if (!comment.trim()) {
      setError('处理意见为必填项（驳回与受理都需填写并留痕）');
      return;
    }
    const payload: ReviewDecisionInput = {
      action: action as ReviewDecisionInput['action'],
      comment: comment.trim(),
    };
    if (action === SCORE_REVIEW_ACTION.APPROVE) {
      const score = Number(correctedScore);
      if (Number.isNaN(score) || score < 0) {
        setError('请输入合法的更正总分（≥0）');
        return;
      }
      payload.corrected_score = score;
      payload.passed_override = passedOverride === '' ? null : passedOverride === 'true';
    }
    setSubmitting(true);
    setError('');
    try {
      await scoreReviewApi.decide(target.id, payload);
      alert(action === SCORE_REVIEW_ACTION.APPROVE ? '已受理并更正成绩' : '已驳回该复核申请');
      closeDialog();
      reload();
    } catch (err) {
      // 重复处理/越权/终态等错误由后端返回，成绩不会被改变
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  const tabs: { key: Tab; label: string }[] = [
    { key: SCORE_REVIEW_STATUS.PENDING as Tab, label: '待处理' },
    { key: SCORE_REVIEW_STATUS.APPROVED as Tab, label: '已受理' },
    { key: SCORE_REVIEW_STATUS.REJECTED as Tab, label: '已驳回' },
    { key: '', label: '全部' },
  ];

  const columns: Column<ScoreReview>[] = [
    { key: 'student_name', title: '学生', render: (r) => <span>{r.student_name}</span> },
    { key: 'exam_title', title: '考试', render: (r) => <span className="font-medium">{r.exam_title}</span> },
    { key: 'reason', title: '申请理由', render: (r) => <span className="line-clamp-1 max-w-[240px] text-sm text-gray-600">{r.reason}</span> },
    { key: 'status', title: '状态', render: (r) => <StatusBadge text={reviewStatusText(r.status)} color={reviewStatusColor(r.status)} /> },
    {
      key: 'score',
      title: '分数变化',
      render: (r) =>
        r.status === SCORE_REVIEW_STATUS.APPROVED ? (
          <span className="text-green-600 text-sm">{r.original_score} → {r.corrected_score}</span>
        ) : (
          <span className="text-xs text-gray-400">-</span>
        ),
    },
    { key: 'created_at', title: '申请时间', render: (r) => <span className="text-xs">{formatDateTime(r.created_at)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (r) => (
        <div className="flex gap-2">
          <Link href={`/records/review?recordId=${r.record_id}`} className="text-brand-600 hover:underline">答卷</Link>
          {r.status === SCORE_REVIEW_STATUS.PENDING && (
            <>
              <button onClick={() => openDialog(r, SCORE_REVIEW_ACTION.APPROVE)} className="text-green-600 hover:underline">受理更正</button>
              <button onClick={() => openDialog(r, SCORE_REVIEW_ACTION.REJECT)} className="text-red-600 hover:underline">驳回</button>
            </>
          )}
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold text-gray-800">成绩复核处理</h1>
      <div className="flex gap-2">
        {tabs.map((t) => (
          <button
            key={t.label}
            onClick={() => { setTab(t.key); setPage(1); }}
            className={`rounded-lg px-3 py-1.5 text-sm ${tab === t.key ? 'bg-brand-600 text-white' : 'border border-gray-300 text-gray-600 hover:bg-gray-50'}`}
          >
            {t.label}
          </button>
        ))}
      </div>
      <DataTable columns={columns} rows={list} loading={loading} emptyTitle="暂无复核申请" />
      <Pagination page={page} pageSize={10} total={total} onChange={setPage} />

      {target && record && (
        <Modal open={!!target} title={action === SCORE_REVIEW_ACTION.APPROVE ? '受理复核并更正成绩' : '驳回复核申请'} onClose={closeDialog} width="max-w-xl">
          <div className="space-y-3 text-sm">
            <div className="rounded-lg bg-gray-50 p-3">
              <p className="font-medium text-gray-800">{target.exam_title} · {target.student_name}</p>
              <p className="mt-1 text-gray-600"><span className="text-gray-400">申请理由：</span>{target.reason}</p>
              <p className="mt-1 text-gray-600">
                当前生效总分：<span className="font-semibold">{record.effective_score}</span>
                {record.score_adjusted && <span className="ml-1 text-xs text-gray-400 line-through">{record.final_score}</span>}
                <span className="ml-3">及格状态：{record.passed ? '及格' : '不及格'}</span>
              </p>
            </div>

            {action === SCORE_REVIEW_ACTION.APPROVE && (
              <div className="grid gap-3 sm:grid-cols-2">
                <label className="block">
                  <span className="text-gray-500">更正后总分</span>
                  <input
                    type="number"
                    min={0}
                    step={0.5}
                    value={correctedScore}
                    onChange={(e) => setCorrectedScore(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-1.5"
                  />
                </label>
                <label className="block">
                  <span className="text-gray-500">及格状态</span>
                  <select
                    value={passedOverride}
                    onChange={(e) => setPassedOverride(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-1.5"
                  >
                    <option value="">按更正分与及格线自动判定</option>
                    <option value="true">教师判定：及格</option>
                    <option value="false">教师判定：不及格</option>
                  </select>
                </label>
              </div>
            )}

            <label className="block">
              <span className="text-gray-500">处理意见（必填，全程留痕）</span>
              <textarea
                value={comment}
                onChange={(e) => setComment(e.target.value)}
                maxLength={500}
                rows={3}
                placeholder={action === SCORE_REVIEW_ACTION.APPROVE ? '说明更正依据，如标准答案配置错误、漏批主观题等' : '说明驳回理由，维持原成绩'}
                className="mt-1 w-full rounded-lg border border-gray-300 px-3 py-2"
              />
            </label>

            {error && <p className="text-sm text-red-600">{error}</p>}
            <div className="flex justify-end gap-2">
              <button onClick={closeDialog} className="rounded-lg border border-gray-300 px-4 py-2 hover:bg-gray-50">取消</button>
              <button
                onClick={submit}
                disabled={submitting}
                className={`rounded-lg px-5 py-2 text-white disabled:opacity-60 ${action === SCORE_REVIEW_ACTION.APPROVE ? 'bg-green-600 hover:bg-green-700' : 'bg-red-600 hover:bg-red-700'}`}
              >
                {submitting ? '提交中…' : action === SCORE_REVIEW_ACTION.APPROVE ? '确认受理并更正' : '确认驳回'}
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
