import { useState } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { DividendTable } from '../features/dividends/components/DividendTable';
import { dividendsService } from '../features/dividends/services/dividendsService';
import { dividendViews, type DividendViewKey } from '../features/dividends/types/views';
import { describeError } from '../shared/api/errors';

export const Route = createFileRoute('/')({
  component: DividendCalendar,
});

function DividendCalendar() {
  const [view, setView] = useState<DividendViewKey>('active');
  const active = dividendViews[view];

  const { data, isPending, error } = useQuery({
    queryKey: ['dividends', view],
    queryFn: () =>
      dividendsService.listDividends({
        filter: active.filter,
        orderBy: active.orderBy,
        pageSize: 50,
      }),
  });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Dividend calendar</h1>
        <p className="mt-1 opacity-70">{active.description}</p>
      </div>

      <div role="tablist" className="tabs tabs-box w-fit">
        {(Object.keys(dividendViews) as DividendViewKey[]).map((key) => (
          <button
            key={key}
            type="button"
            role="tab"
            className={`tab ${key === view ? 'tab-active' : ''}`}
            onClick={() => setView(key)}
          >
            {dividendViews[key].label}
          </button>
        ))}
      </div>

      {isPending && <div className="rounded-box bg-base-100 p-12 text-center">Loading…</div>}

      {error && (
        <div role="alert" className="alert alert-error">
          <span>{describeError(error)}</span>
        </div>
      )}

      {data && (
        <>
          <DividendTable dividends={data.dividends} />
          <p className="text-sm opacity-60">
            Showing {data.dividends.length} of {data.totalSize}.
          </p>
        </>
      )}
    </div>
  );
}
