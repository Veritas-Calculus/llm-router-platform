// Shared currency/number formatting helpers.
//
// Use these instead of inlining `Intl.NumberFormat(...)` in pages — they keep
// decimal rules, currency symbols, and locale fallbacks consistent across the
// admin financial dashboard, the user usage page, and any payment UI.
//
// Conventions:
//  - cards / aggregate totals → 2 decimals (matches printed receipts)
//  - per-request line items / ledger detail → 4 decimals (sub-cent precision)
//  - large absolute counts (requests, tokens) → compact (1.5M, 25K)

const CURRENCY_FRACTION_2 = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const CURRENCY_FRACTION_4 = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 4,
  maximumFractionDigits: 4,
});

const NUMBER = new Intl.NumberFormat('en-US');

const COMPACT = new Intl.NumberFormat('en-US', {
  notation: 'compact',
  maximumFractionDigits: 1,
});

export type MoneyValue = number | string | null | undefined;

export function moneyNumber(amount: MoneyValue): number {
  if (amount == null || amount === '') return 0;
  const parsed = typeof amount === 'number' ? amount : Number(amount);
  return Number.isFinite(parsed) ? parsed : 0;
}

export function formatUSD(amount: MoneyValue): string {
  return CURRENCY_FRACTION_2.format(moneyNumber(amount));
}

export function formatUSDPrecise(amount: MoneyValue): string {
  return CURRENCY_FRACTION_4.format(moneyNumber(amount));
}

export function formatNumber(n: number | null | undefined): string {
  if (n == null || Number.isNaN(n)) return '0';
  return NUMBER.format(n);
}

export function formatCompact(n: number | null | undefined): string {
  if (n == null || Number.isNaN(n)) return '0';
  return COMPACT.format(n);
}

export function formatPercent(value: number | null | undefined, fractionDigits = 1): string {
  if (value == null || Number.isNaN(value)) return '0%';
  return `${value.toFixed(fractionDigits)}%`;
}
