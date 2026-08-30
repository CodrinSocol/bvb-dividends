import { Link } from 'react-router-dom';
import type { Dividend } from '../api/client';
import { companyOf, describeState, formatDate, formatMoney } from '../lib/format';

export function DividendTable({ dividends }: { dividends: Dividend[] }) {
  if (dividends.length === 0) {
    return (
      <div className="rounded-box bg-base-100 p-12 text-center opacity-70">
        No dividends match this view.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto rounded-box bg-base-100">
      <table className="table">
        <thead>
          <tr>
            <th>Company</th>
            <th>Year</th>
            <th className="text-right">Per share</th>
            <th>Ex-date</th>
            <th>Payment</th>
            <th>State</th>
          </tr>
        </thead>
        <tbody>
          {dividends.map((dividend) => {
            const symbol = companyOf(dividend.name);
            const state = describeState(dividend.state);
            return (
              <tr key={dividend.name}>
                <td>
                  <Link to={`/companies/${symbol}`} className="link link-hover font-medium">
                    {symbol}
                  </Link>
                </td>
                <td>{dividend.year ?? '—'}</td>
                <td className="text-right font-mono">
                  {formatMoney(dividend.grossPerShareNaturalPerson)}
                </td>
                <td>{formatDate(dividend.schedule?.exDividendDate)}</td>
                <td>{formatDate(dividend.schedule?.paymentStartDate)}</td>
                <td>
                  <span className={`badge ${state.className}`}>{state.label}</span>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
