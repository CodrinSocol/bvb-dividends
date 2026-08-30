import { createFileRoute } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { DividendTable } from '../../features/dividends/components/DividendTable';
import { dividendsService } from '../../features/dividends/services/dividendsService';
import { describeError, isNotFound } from '../../shared/api/errors';

export const Route = createFileRoute('/companies/$symbol')({
  component: CompanyDividends,
});

function CompanyDividends() {
  const { symbol } = Route.useParams();

  const { data, isPending, error } = useQuery({
    queryKey: ['dividends', 'company', symbol],
    queryFn: () =>
      dividendsService.listDividends({
        company: symbol,
        orderBy: 'schedule.ex_dividend_date desc',
        pageSize: 100,
      }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">{symbol}</h1>

      {isPending && <div className="rounded-box bg-base-100 p-12 text-center">Loading…</div>}

      {error && (
        <div role="alert" className="alert alert-error">
          <span>
            {isNotFound(error)
              ? `BVB has no company with the symbol ${symbol}.`
              : describeError(error)}
          </span>
        </div>
      )}

      {data && (
        <>
          <DividendTable dividends={data.dividends} />
          <p className="text-sm opacity-60">{data.totalSize} dividends on record.</p>
        </>
      )}
    </div>
  );
}
