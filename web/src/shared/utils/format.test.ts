import { describe, expect, it } from 'vitest';
import { create } from '@bufbuild/protobuf';
import { MoneySchema } from '@bvb-dividends/gen/google/type/money_pb';
import { DateSchema } from '@bvb-dividends/gen/google/type/date_pb';
import { companyOf, formatDate, formatMoney } from './format';

describe('formatMoney', () => {
  // google.type.Money splits a value into an int64 of whole units and a nanos
  // remainder, so both halves have to be recombined. Reading only the units
  // would silently drop every fractional dividend, which is most of them.
  it('recombines units and nanos', () => {
    const money = create(MoneySchema, { currencyCode: 'RON', units: 2n, nanos: 345_000_000 });
    expect(formatMoney(money, 'en-GB')).toContain('2.345');
  });

  it('reads a value smaller than one unit', () => {
    const money = create(MoneySchema, { currencyCode: 'RON', units: 0n, nanos: 34_500_000 });
    expect(formatMoney(money, 'en-GB')).toContain('0.0345');
  });

  // An amount BVB has not reported is absent, not zero: a dividend still being
  // approved must not read as one that pays nothing.
  it('shows an unreported amount as absent', () => {
    expect(formatMoney(undefined)).toBe('—');
  });
});

describe('formatDate', () => {
  it('formats a calendar date without shifting it across a time zone', () => {
    const date = create(DateSchema, { year: 2026, month: 4, day: 17 });
    expect(formatDate(date, 'en-GB')).toBe('17 Apr 2026');
  });

  it('shows an unreported date as absent', () => {
    expect(formatDate(undefined)).toBe('—');
    expect(formatDate(create(DateSchema, {}))).toBe('—');
  });
});

describe('companyOf', () => {
  it('reads the symbol out of a resource name', () => {
    expect(companyOf('companies/SNP/dividends/8f14e45f-ceea-5a04-a6f3-2f4b1a9c0d33')).toBe('SNP');
    expect(companyOf('companies/TLV')).toBe('TLV');
    expect(companyOf(undefined)).toBe('');
  });
});
