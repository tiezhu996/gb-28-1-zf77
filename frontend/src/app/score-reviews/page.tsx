'use client';
import { Suspense, useCallback, useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useScoreReviewStore } from '@/stores/scoreReviewStore';
import { DataTable, type Column } from '@/components/DataTable';
import { Pagination } from '@/components/Pagination';
import { StatusBadge } from '@/components/StatusBadge';
import { formatDateTime, reviewStatusColor, reviewStatusText } from '@/utils/format';
import type { ScoreReview } from '@/types';

function ScoreReviews() {
  const router = useRouter();
  const params = useSearchParams();
  const examId = params.get('examId') ?? '';
  const { list, total, loading, fetchAll } = useScoreReviewStore();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('');

  const reload = useCallback(() => {
    fetchAll({ status, exam_id: examId || undefined, page, page_size: 10 });
  }, [fetchAll, status, examId, page]);

  useEffect(() => {
    reload();
  }, [reload]);

  const columns: Column<ScoreReview>[] = [
    { key: 'exam_title', title: '考试', render: (r) => <span className="font-medium">{r.exam_title}</span> },
    { key: 'student_name', title: '学生', render: (r) => <span>{r.student_name}</span> },
    {
      key: 'status',
      title: '状态',
      render: (r) => <StatusBadge text={reviewStatusText(r.status)} color={reviewStatusColor(r.status)} />,
    },
    {
      key: 'original_score',
      title: '原成绩',
      render: (r) => (
        <span>
          {r.original_score}
          <span className={`ml-1 text-[10px] ${r.original_passed ? 'text-green-600' : 'text-red-600'}`}>
            {r.original_passed ? '及格' : '不及格'}
          </span>
        </span>
      ),
    },
    {
      key: 'corrected_score',
      title: '最终成绩',
      render: (r) =>
        r.status === 'approved' ? (
          <span>
            <span className={r.score_corrected ? 'font-semibold text-orange-600' : ''}>
              {r.score_corrected ? r.corrected_score : r.original_score}
            </span>
            <span className={`ml-1 text-[10px] ${r.corrected_passed ? 'text-green-600' : 'text-red-600'}`}>
              {r.corrected_passed ? '及格' : '不及格'}
            </span>
          </span>
        ) : (
          <span className="text-xs text-gray-400">-</span>
        ),
    },
    { key: 'reason', title: '申请理由', render: (r) => <span className="line-clamp-1 max-w-[16rem] text-xs text-gray-600">{r.reason}</span> },
    { key: 'created_at', title: '申请时间', render: (r) => <span className="text-xs">{formatDateTime(r.created_at)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (r) => (
        <button
          onClick={() => router.push(`/records/review?recordId=${r.record_id}`)}
          className="text-brand-600 hover:underline"
        >
          {r.status === 'pending' ? '处理复核' : '查看详情'}
        </button>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-xl font-bold text-gray-800">成绩复核处理</h1>
        <select
          value={status}
          onChange={(e) => { setStatus(e.target.value); setPage(1); }}
          className="rounded-lg border border-gray-300 px-2 py-1.5 text-sm"
        >
          <option value="">全部状态</option>
          <option value="pending">待处理</option>
          <option value="approved">已受理</option>
          <option value="rejected">已驳回</option>
        </select>
      </div>
      <p className="text-xs text-gray-400">
        学生可在批改完成后 48 小时内发起一次复核；受理可更正总分与及格状态，受理/驳回均须填写意见并全程留痕。
      </p>
      <DataTable columns={columns} rows={list} loading={loading} emptyTitle="暂无成绩复核申请" />
      <Pagination page={page} pageSize={10} total={total} onChange={setPage} />
    </div>
  );
}

export default function ScoreReviewsPage() {
  return (
    <Suspense fallback={<div className="p-10 text-center text-gray-400">加载中…</div>}>
      <ScoreReviews />
    </Suspense>
  );
}
