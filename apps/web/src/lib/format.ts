import type { GoogleDate, Money } from '../api/client';

/**
 * google.type.Money carries whole units as a string, because the field is an
 * int64 and JSON numbers cannot hold one safely, plus a nanos remainder. Both
 * halves have to be recombined to get the value back.
 */
export function formatMoney(money: Money | undefined, locale = 'ro-RO'): string {
  if (!money) return '—';

  const units = BigInt(money.units ?? '0');
  const nanos = money.nanos ?? 0;
  const value = Number(units) + nanos / 1_000_000_000;

  return new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: money.currencyCode || 'RON',
    minimumFractionDigits: 2,
    maximumFractionDigits: 4,
  }).format(value);
}

/** Formats a google.type.Date, which is a calendar date with no time zone. */
export function formatDate(date: GoogleDate | undefined, locale = 'ro-RO'): string {
  if (!date?.year || !date.month || !date.day) return '—';
  return new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    timeZone: 'UTC',
  }).format(new Date(Date.UTC(date.year, date.month - 1, date.day)));
}

/** The lifecycle states the API reports, as labels and badge styles. */
const stateLabels: Record<string, { label: string; className: string }> = {
  STATE_ANNOUNCED: { label: 'Announced', className: 'badge-success' },
  STATE_EX_PASSED: { label: 'Ex-date passed', className: 'badge-warning' },
  STATE_PAYING: { label: 'Paying', className: 'badge-info' },
  STATE_PAID: { label: 'Paid', className: 'badge-ghost' },
  STATE_UNDATED: { label: 'Not scheduled', className: 'badge-outline' },
};

export function describeState(state: string | undefined) {
  return stateLabels[state ?? ''] ?? { label: 'Unknown', className: 'badge-ghost' };
}

/** Extracts the company symbol from a `companies/{symbol}/...` resource name. */
export function companyOf(resourceName: string | undefined): string {
  return resourceName?.split('/')[1] ?? '';
}
