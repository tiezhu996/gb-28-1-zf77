'use client';
import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { useRecordStore } from '@/stores/recordStore';
import { DataTable, type Column } from '@/components/DataTable';
import { Pagination } from '@/components/Pagination';
import { StatusBadge } from '@/components/StatusBadge';
import { formatDateTime, recordStatusColor, recordStatusText, reviewStatusColor, reviewStatusText } from '@/utils/format';
import { SCORE_REVIEW_STATUS, REVIEW_WINDOW_MS } from '@/constants';
import type { ExamRecord } from '@/types';

export default function RecordsPage() {
  const { mine, total, loading, fetchMine } = useRecordStore();
  const [page, setPage] = useState(1);

  const reload = useCallback(() => {
    fetchMine({ page, page_size: 10 });
  }, [fetchMine, page]);

  useEffect(() => {
    reload();
  }, [reload]);

  // 是否仍在批改完成后的 48 小时复核窗口内
  const inReviewWindow = (r: ExamRecord) =>
    !!r.graded_at && Date.now() - new Date(r.graded_at).getTime() <= REVIEW_WINDOW_MS;

  const columns: Column<ExamRecord>[] = [
    { key: 'exam_title', title: '考试', render: (r) => <span className="font-medium">{r.exam_title}</span> },
    { key: 'status', title: '状态', render: (r) => <StatusBadge text={recordStatusText(r.status)} color={recordStatusColor(r.status)} /> },
    { key: 'objective_score', title: '客观题分', render: (r) => <span>{r.objective_score}</span> },
    {
      key: 'effective_score',
      title: '最终分',
      render: (r) => (
        <span className="inline-flex items-center gap-1">
          <span className={`font-semibold ${r.score_adjusted ? 'text-green-600' : ''}`}>
            {r.status === 'graded' ? r.effective_score : '-'}
          </span>
          {r.score_adjusted && (
            <span className="text-xs text-gray-400 line-through">{r.final_score}</span>
          )}
        </span>
      ),
    },
    {
      key: 'passed',
      title: '及格',
      render: (r) =>
        r.status === 'graded' ? (
          <StatusBadge text={r.passed ? '及格' : '不及格'} color={r.passed ? 'green' : 'red'} />
        ) : (
          <span>-</span>
        ),
    },
    {
      key: 'review',
      title: '复核状态',
      render: (r) =>
        r.review ? (
          <StatusBadge text={reviewStatusText(r.review.status)} color={reviewStatusColor(r.review.status)} />
        ) : r.status === 'graded' && inReviewWindow(r) ? (
          <StatusBadge text="可申请复核" color="orange" />
        ) : (
          <span className="text-xs text-gray-400">-</span>
        ),
    },
    { key: 'started_at', title: '开始时间', render: (r) => <span className="text-xs">{formatDateTime(r.started_at)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (r) => (
        <div className="flex gap-2">
          {r.status === 'in_progress' ? (
            <Link href={`/exam-take?recordId=${r.id}`} className="text-brand-600 hover:underline">继续作答</Link>
          ) : (
            <Link href={`/records/review?recordId=${r.id}`} className="text-brand-600 hover:underline">查看答卷</Link>
          )}
          {/* 仅在 48h 窗口内、且无任何复核申请时显示申请入口（其余由后端强校验） */}
          {r.status === 'graded' && inReviewWindow(r) && !r.review && (
            <Link href={`/reviews/apply?recordId=${r.id}`} className="text-orange-600 hover:underline">申请复核</Link>
          )}
          {r.review && r.review.status === SCORE_REVIEW_STATUS.PENDING && (
            <Link href={`/reviews/apply?recordId=${r.id}`} className="text-orange-600 hover:underline">复核详情</Link>
          )}
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-gray-800">我的考试记录</h1>
        <Link href="/reviews/mine" className="text-sm text-brand-600 hover:underline">我的复核申请</Link>
      </div>
      <p className="text-xs text-gray-400">批改完成后 48 小时内可对本人成绩发起一次复核；同一答卷仅允许一条申请，逾期或已申请将被拒绝。</p>
      <DataTable columns={columns} rows={mine} loading={loading} emptyTitle="还没有参加过考试" />
      <Pagination page={page} pageSize={10} total={total} onChange={setPage} />
    </div>
  );
}
