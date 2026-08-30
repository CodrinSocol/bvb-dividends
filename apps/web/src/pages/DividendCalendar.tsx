import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ApiError, listDividends } from '../api/client';
import { DividendTable } from '../components/DividendTable';

/**
 * The views the calendar offers, each expressed as an AIP-160 filter.
 *
 * "Active" is the endpoint the Java service exposed as /active-dividends. Under
 * the AIPs it is not a method of its own but a filter over the collection,
 * which is why the API has one fewer endpoint and strictly more capability.
 */
const views = {
  active: {
    label: 'Still claimable',
    description: 'The ex-dividend date has not passed, so buying today still earns the dividend.',
    filter: `schedule.ex_dividend_date > "${new Date().toISOString().slice(0, 10)}"`,
    orderBy: 'schedule.ex_dividend_date',
  },
  all: {
    label: 'All dividends',
    description: 'Everything imported from BVB, most recent ex-date first.',
    filter: '',
    orderBy: 'schedule.ex_dividend_date desc',
  },
  unscheduled: {
    label: 'Announced, not scheduled',
    description: 'BVB has announced these but has not published an ex-dividend date yet.',
    filter: 'schedule.ex_dividend_date = null',
    orderBy: 'year desc',
  },
} as const;

type ViewKey = keyof typeof views;

export function DividendCalendar() {
  const [view, setView] = useState<ViewKey>('active');
  const active = views[view];

  const { data, isPending, error } = useQuery({
    queryKey: ['dividends', view],
    queryFn: () => listDividends({ filter: active.filter, orderBy: active.orderBy, pageSize: 50 }),
  });

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Dividend calendar</h1>
        <p className="mt-1 opacity-70">{active.description}</p>
      </div>

      <div role="tablist" className="tabs tabs-box w-fit">
        {(Object.keys(views) as ViewKey[]).map((key) => (
          <button
            key={key}
            type="button"
            role="tab"
            className={`tab ${key === view ? 'tab-active' : ''}`}
            onClick={() => setView(key)}
          >
            {views[key].label}
          </button>
        ))}
      </div>

      {isPending && <div className="rounded-box bg-base-100 p-12 text-center">Loading…</div>}

      {error && (
        <div role="alert" className="alert alert-error">
          <span>
            {error instanceof ApiError
              ? `${error.status}: ${error.message}`
              : 'The API could not be reached.'}
          </span>
        </div>
      )}

      {data && (
        <>
          <DividendTable dividends={data.dividends ?? []} />
          <p className="text-sm opacity-60">
            Showing {data.dividends?.length ?? 0} of {data.totalSize ?? 0}.
          </p>
        </>
      )}
    </div>
  );
}
