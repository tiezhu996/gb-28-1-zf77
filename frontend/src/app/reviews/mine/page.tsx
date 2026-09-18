'use client';
import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { scoreReviewApi } from '@/api/scoreReview';
import { DataTable, type Column } from '@/components/DataTable';
import { Pagination } from '@/components/Pagination';
import { StatusBadge } from '@/components/StatusBadge';
import { formatDateTime, reviewStatusColor, reviewStatusText } from '@/utils/format';
import { SCORE_REVIEW_STATUS } from '@/constants';
import type { ScoreReview } from '@/types';

export default function MyReviewsPage() {
  const [list, setList] = useState<ScoreReview[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const res = await scoreReviewApi.mine({ page, page_size: 10 });
      setList(res.list);
      setTotal(res.total);
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    reload();
  }, [reload]);

  const columns: Column<ScoreReview>[] = [
    { key: 'exam_title', title: '考试', render: (r) => <span className="font-medium">{r.exam_title}</span> },
    { key: 'reason', title: '复核理由', render: (r) => <span className="line-clamp-1 text-sm text-gray-600">{r.reason}</span> },
    { key: 'status', title: '状态', render: (r) => <StatusBadge text={reviewStatusText(r.status)} color={reviewStatusColor(r.status)} /> },
    {
      key: 'score',
      title: '最终分',
      render: (r) =>
        r.status === SCORE_REVIEW_STATUS.APPROVED ? (
          <span className="text-green-600">
            {r.corrected_score}
            <span className="ml-1 text-xs text-gray-400 line-through">{r.original_score}</span>
          </span>
        ) : (
          <span className="text-gray-400">-</span>
        ),
    },
    { key: 'handler', title: '处理教师', render: (r) => <span className="text-sm">{r.handler_name || '-'}</span> },
    { key: 'created_at', title: '申请时间', render: (r) => <span className="text-xs">{formatDateTime(r.created_at)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (r) => (
        <Link href={`/reviews/apply?recordId=${r.record_id}`} className="text-brand-600 hover:underline">查看详情</Link>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold text-gray-800">我的复核申请</h1>
      <DataTable columns={columns} rows={list} loading={loading} emptyTitle="暂无复核申请" />
      <Pagination page={page} pageSize={10} total={total} onChange={setPage} />
    </div>
  );
}
