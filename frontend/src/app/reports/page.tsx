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

  const review = report.review_stats;

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

      {review && review.total_count > 0 && (
        <section className="rounded-xl border border-gray-200 bg-white p-5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="font-semibold text-gray-800">成绩复核统计</h2>
            <p className="text-xs text-gray-400">平均分/及格率/分数段均已按复核更正后的最终分数计算</p>
          </div>
          <div className="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div className="rounded-lg bg-gray-50 p-3 text-center">
              <p className="text-xs text-gray-400">复核总数</p>
              <p className="mt-1 text-xl font-bold text-gray-700">{review.total_count}</p>
            </div>
            <div className="rounded-lg bg-orange-50 p-3 text-center">
              <p className="text-xs text-orange-400">待处理</p>
              <p className="mt-1 text-xl font-bold text-orange-600">{review.pending_count}</p>
            </div>
            <div className="rounded-lg bg-green-50 p-3 text-center">
              <p className="text-xs text-green-400">已受理（更正）</p>
              <p className="mt-1 text-xl font-bold text-green-600">{review.approved_count}</p>
            </div>
            <div className="rounded-lg bg-red-50 p-3 text-center">
              <p className="text-xs text-red-400">已驳回</p>
              <p className="mt-1 text-xl font-bold text-red-600">{review.rejected_count}</p>
            </div>
          </div>
          {review.adjusted_records.length > 0 && (
            <div className="mt-4">
              <h3 className="text-sm font-medium text-gray-700">经复核更正的成绩</h3>
              <div className="mt-2 space-y-2">
                {review.adjusted_records.map((a) => (
                  <div key={a.record_id} className="flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 px-3 py-2 text-sm">
                    <span className="w-24 shrink-0">{a.student_name}</span>
                    <span className="text-gray-400 line-through">{a.original_score}</span>
                    <span className="text-gray-400">→</span>
                    <span className="font-semibold text-green-600">{a.corrected_score}</span>
                    <span className={`text-xs ${a.corrected_passed ? 'text-green-600' : 'text-red-600'}`}>
                      {a.corrected_passed ? '及格' : '不及格'}
                    </span>
                    <span className="flex-1 truncate text-xs text-gray-400">{a.comment}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </section>
      )}

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
