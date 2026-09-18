'use client';
import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { useRecordStore } from '@/stores/recordStore';
import { DataTable, type Column } from '@/components/DataTable';
import { Pagination } from '@/components/Pagination';
import { StatusBadge } from '@/components/StatusBadge';
import { formatDateTime, recordStatusColor, recordStatusText, reviewStatusColor, reviewStatusText } from '@/utils/format';
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

  const columns: Column<ExamRecord>[] = [
    { key: 'exam_title', title: '考试', render: (r) => <span className="font-medium">{r.exam_title}</span> },
    { key: 'status', title: '状态', render: (r) => <StatusBadge text={recordStatusText(r.status)} color={recordStatusColor(r.status)} /> },
    {
      key: 'review_status',
      title: '复核',
      render: (r) =>
        r.review_status && r.review_status !== 'none' ? (
          <StatusBadge text={reviewStatusText(r.review_status)} color={reviewStatusColor(r.review_status)} />
        ) : (
          <span className="text-xs text-gray-400">-</span>
        ),
    },
    { key: 'objective_score', title: '客观题分', render: (r) => <span>{r.objective_score}</span> },
    {
      key: 'final_score',
      title: '最终分',
      render: (r) => (
        <span>
          <span className={`font-semibold ${r.score_corrected ? 'text-orange-600' : ''}`}>{r.final_score || '-'}</span>
          {r.score_corrected && <span className="ml-1 text-[10px] text-orange-500">已更正</span>}
          {r.pass_score > 0 && r.status === 'graded' && (
            <span className={`ml-1 text-[10px] ${r.is_passed ? 'text-green-600' : 'text-red-600'}`}>
              {r.is_passed ? '及格' : '不及格'}
            </span>
          )}
        </span>
      ),
    },
    { key: 'cheat_count', title: '切屏次数', render: (r) => <span className={r.cheat_count > 0 ? 'text-red-600' : ''}>{r.cheat_count}</span> },
    { key: 'started_at', title: '开始时间', render: (r) => <span className="text-xs">{formatDateTime(r.started_at)}</span> },
    { key: 'actions', title: '操作', render: (r) => (
        <div className="flex gap-2">
          {r.status === 'in_progress' ? (
            <Link href={`/exam-take?recordId=${r.id}`} className="text-brand-600 hover:underline">继续作答</Link>
          ) : (
            <Link href={`/records/review?recordId=${r.id}`} className="text-brand-600 hover:underline">查看答卷</Link>
          )}
        </div>
      ) },
  ];

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold text-gray-800">我的考试记录</h1>
      <p className="text-xs text-gray-400">批改完成后 48 小时内可在答卷详情页发起一次成绩复核；最终分为复核受理后的最终结果。</p>
      <DataTable columns={columns} rows={mine} loading={loading} emptyTitle="还没有参加过考试" />
      <Pagination page={page} pageSize={10} total={total} onChange={setPage} />
    </div>
  );
}
