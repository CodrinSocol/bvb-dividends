import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ApiError, listDividends } from '../api/client';
import { DividendTable } from '../components/DividendTable';

export function CompanyDividends() {
  const { symbol = '' } = useParams<{ symbol: string }>();

  const { data, isPending, error } = useQuery({
    queryKey: ['dividends', 'company', symbol],
    queryFn: () =>
      listDividends({ company: symbol, orderBy: 'schedule.ex_dividend_date desc', pageSize: 100 }),
    enabled: symbol !== '',
  });

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">{symbol}</h1>

      {isPending && <div className="rounded-box bg-base-100 p-12 text-center">Loading…</div>}

      {error && (
        <div role="alert" className="alert alert-error">
          <span>
            {error instanceof ApiError && error.code === 404
              ? `BVB has no company with the symbol ${symbol}.`
              : 'The API could not be reached.'}
          </span>
        </div>
      )}

      {data && (
        <>
          <DividendTable dividends={data.dividends ?? []} />
          <p className="text-sm opacity-60">{data.totalSize ?? 0} dividends on record.</p>
        </>
      )}
    </div>
  );
}
