import { Dividend_State } from '@bvb-dividends/gen/bvb/dividends/v1/dividends_pb';

/**
 * How each lifecycle stage is shown.
 *
 * The stage is derived by the service from the dividend's schedule and today,
 * so it is never stale, and it arrives as an enum rather than as a string: a
 * stage this build does not know about is a compile error rather than a badge
 * that silently reads "Unknown".
 */
const labels: Record<Dividend_State, { label: string; className: string }> = {
  [Dividend_State.UNSPECIFIED]: { label: 'Unknown', className: 'badge-ghost' },
  [Dividend_State.ANNOUNCED]: { label: 'Announced', className: 'badge-success' },
  [Dividend_State.EX_PASSED]: { label: 'Ex-date passed', className: 'badge-warning' },
  [Dividend_State.PAYING]: { label: 'Paying', className: 'badge-info' },
  [Dividend_State.PAID]: { label: 'Paid', className: 'badge-ghost' },
  [Dividend_State.UNDATED]: { label: 'Not scheduled', className: 'badge-outline' },
};

export function describeState(state: Dividend_State): { label: string; className: string } {
  return labels[state] ?? labels[Dividend_State.UNSPECIFIED];
}
