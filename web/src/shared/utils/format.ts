import type { Money } from '@bvb-dividends/gen/google/type/money_pb';
import type { Date as GoogleDate } from '@bvb-dividends/gen/google/type/date_pb';

/**
 * google.type.Money carries whole units as an int64 and a nanos remainder, so
 * both halves have to be recombined to get the value back. The generated client
 * decodes the int64 as a bigint, which is why it is converted rather than read.
 *
 * An absent amount is one BVB has not reported, which is not the same as zero
 * and must not read as it.
 */
export function formatMoney(money: Money | undefined, locale = 'ro-RO'): string {
  if (!money) return '—';

  const value = Number(money.units) + money.nanos / 1_000_000_000;

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


/** Extracts the company symbol from a `companies/{symbol}/...` resource name. */
export function companyOf(resourceName: string | undefined): string {
  return resourceName?.split('/')[1] ?? '';
}

/** Today, as the YYYY-MM-DD literal an AIP-160 date filter compares against. */
export function todayFilterLiteral(now = new Date()): string {
  return now.toISOString().slice(0, 10);
}
