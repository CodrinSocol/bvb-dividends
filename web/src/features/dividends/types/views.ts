import { todayFilterLiteral } from '../../../shared/utils/format';

/**
 * The views the calendar offers, each expressed as an AIP-160 filter.
 *
 * "Still claimable" is the endpoint the Java service exposed as
 * /active-dividends. Under the AIPs it is not a method of its own but a filter
 * over the collection, which is why this API has one fewer endpoint and
 * strictly more reach: "announced but not yet scheduled" is a question the old
 * API could not express at all, because its date-range queries excluded every
 * dividend with no ex-dividend date.
 */
export const dividendViews = {
  active: {
    label: 'Still claimable',
    description: 'The ex-dividend date has not passed, so buying today still earns the dividend.',
    filter: 'state = "STATE_ANNOUNCED"',
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

export type DividendViewKey = keyof typeof dividendViews;

/** Today's date, for a view that compares against it. */
export const today = todayFilterLiteral;
