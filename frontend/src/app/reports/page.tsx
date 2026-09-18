'use client';
import { Suspense, useEffect, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { recordApi } from '@/api/record';
import { questionTypeText } from '@/utils/format';
import type { ExamReport } from '@/types';

function Report() {
  const params = useSearchParams();
  const examId = params.get('examId') ?? '';
  const [report, setReport] = useState<ExamReport | null>(null);

  useEffect(() => {
    if (examId) {
      recordApi.report(examId).then(setReport).catch(() => undefined);
    }
  }, [examId]);

  if (!report) return <div className="p-10 text-center text-gray-400">加载中…</div>;

  const maxBand = Math.max(1, ...Object.values(report.score_bands));

  const stats = [
    { label: '参考人数', value: report.total_students },
    { label: '平均分', value: report.average_score },
    { label: '最高分', value: report.max_score },
    { label: '最低分', value: report.min_score },
    { label: '及格率', value: `${report.pass_rate}%` },
  ];

  const reviewStats = [
    { label: '待处理复核', value: report.review_pending_count ?? 0, color: 'text-orange-600' },
    { label: '已受理', value: report.review_approved_count ?? 0, color: 'text-green-600' },
    { label: '已驳回', value: report.review_rejected_count ?? 0, color: 'text-red-600' },
    { label: '受理并改分', value: report.review_corrected_count ?? 0, color: 'text-brand-600' },
  ];

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold text-gray-800">成绩分析 · {report.exam_title}</h1>

      <section className="grid grid-cols-2 gap-3 sm:grid-cols-5">
        {stats.map((s) => (
          <div key={s.label} className="rounded-xl border border-gray-200 bg-white p-4 text-center">
            <p className="text-xs text-gray-400">{s.label}</p>
            <p className="mt-1 text-2xl font-bold text-brand-600">{s.value}</p>
          </div>
        ))}
      </section>

      <section className="rounded-xl border border-gray-200 bg-white p-5">
        <div className="flex items-center justify-between">
          <h2 className="font-semibold text-gray-800">成绩复核统计</h2>
          <p className="text-xs text-gray-400">平均分/及格率/分数段均以复核受理后的最终分为准</p>
        </div>
        <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
          {reviewStats.map((s) => (
            <div key={s.label} className="rounded-lg bg-gray-50 p-3 text-center">
              <p className="text-xs text-gray-400">{s.label}</p>
              <p className={`mt-1 text-xl font-bold ${s.color}`}>{s.value}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="rounded-xl border border-gray-200 bg-white p-5">
        <h2 className="font-semibold text-gray-800">分数段分布</h2>
        <div className="mt-4 space-y-2">
          {Object.entries(report.score_bands).map(([band, count]) => (
            <div key={band} className="flex items-center gap-3">
              <span className="w-14 text-xs text-gray-500">{band}</span>
              <div className="h-5 flex-1 rounded bg-gray-100">
                <div
                  className="h-5 rounded bg-gradient-to-r from-brand-500 to-brand-600"
                  style={{ width: `${(count / maxBand) * 100}%` }}
                />
              </div>
              <span className="w-8 text-xs text-gray-500">{count}人</span>
            </div>
          ))}
        </div>
      </section>

      <section className="rounded-xl border border-gray-200 bg-white p-5">
        <h2 className="font-semibold text-gray-800">每题正确率分析</h2>
        <div className="mt-4 space-y-2">
          {report.question_reports.map((q) => (
            <div key={q.question_id} className="flex items-center gap-3 rounded-lg bg-gray-50 px-3 py-2">
              <span className="w-14 shrink-0 text-xs text-gray-400">{questionTypeText(q.type)}</span>
              <span className="flex-1 truncate text-sm">{q.content}</span>
              <span className="shrink-0 text-xs text-gray-400">答对 {q.correct_count}/{q.answer_count}</span>
              <span className={`w-14 shrink-0 text-right text-sm font-medium ${q.accuracy >= 60 ? 'text-green-600' : 'text-red-600'}`}>
                {q.accuracy}%
              </span>
            </div>
          ))}
          {report.question_reports.length === 0 && <p className="text-sm text-gray-400">暂无答题数据</p>}
        </div>
      </section>
    </div>
  );
}

export default function ReportsPage() {
  return (
    <Suspense fallback={<div className="p-10 text-center text-gray-400">加载中…</div>}>
      <Report />
    </Suspense>
  );
}
