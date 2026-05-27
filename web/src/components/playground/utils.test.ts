import { describe, it, expect } from 'vitest';
import { computeTokensPerSec, formatTokensPerSec } from './utils';

// M-03: previously `output / total_ms` would round to 0 tok/s for slow
// streams that produced a single token over 27s. The new formula uses
// `output / (total_ms - ttfb_ms)` which is the actual generation
// throughput, and the renderer drops to "—" when the window is too
// short to time meaningfully.
describe('computeTokensPerSec', () => {
  it('returns post-TTFB throughput, not whole-duration', () => {
    // 27s total, 24s TTFB, 1 output token -> 1 / 3 ≈ 0.333 tok/s
    expect(computeTokensPerSec(1, 24000, 27000)).toBeCloseTo(0.333, 2);
  });

  it('returns NaN when the post-TTFB window is shorter than 100ms', () => {
    expect(Number.isNaN(computeTokensPerSec(50, 990, 1050))).toBe(true);
  });

  it('returns 0 for zero output tokens', () => {
    expect(computeTokensPerSec(0, 500, 1000)).toBe(0);
  });

  it('handles fast generations', () => {
    // 100 tokens over 2s post-TTFB = 50 tok/s
    expect(computeTokensPerSec(100, 1000, 3000)).toBe(50);
  });
});

describe('formatTokensPerSec', () => {
  it('renders "—" for NaN (too fast to measure)', () => {
    expect(formatTokensPerSec(Number.NaN)).toBe('—');
  });

  it('renders "—" for zero', () => {
    expect(formatTokensPerSec(0)).toBe('—');
  });

  it('renders 1 decimal when < 10 (so 0.35 stays visible)', () => {
    expect(formatTokensPerSec(0.333)).toBe('0.3');
    expect(formatTokensPerSec(0.35)).toBe('0.4');
    expect(formatTokensPerSec(9.94)).toBe('9.9');
  });

  it('rounds to integer when >= 10', () => {
    expect(formatTokensPerSec(50)).toBe('50');
    expect(formatTokensPerSec(124.6)).toBe('125');
  });
});
