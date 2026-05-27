import { describe, it, expect } from 'vitest';
import {
  DEFAULT_PASSWORD_POLICY,
  describePolicy,
  validatePassword,
  type PasswordPolicy,
} from './passwordPolicy';

// Mirrors the post-audit P0-2 contract: the client validator advertises and
// enforces every rule the server enforces. A password that meets all five
// character-class rules but appears in the server's common-password list
// (e.g. "Passw0rd1") used to "pass" client-side and surprise the user with
// a server rejection. The new blockCommonPasswords flag closes that gap.

describe('validatePassword', () => {
  it('reports empty password without an error message', () => {
    const r = validatePassword('', DEFAULT_PASSWORD_POLICY);
    expect(r.valid).toBe(false);
    expect(r.message).toBe('');
  });

  it('flags short passwords on the length rule first', () => {
    const r = validatePassword('Aa1', DEFAULT_PASSWORD_POLICY);
    expect(r.message).toMatch(/at least 8 characters/i);
  });

  it('requires an uppercase letter', () => {
    const r = validatePassword('only-lowercase-letters-9', DEFAULT_PASSWORD_POLICY);
    expect(r.message).toMatch(/uppercase/i);
  });

  it('requires a lowercase letter', () => {
    const r = validatePassword('ONLY-UPPERCASE-LETTERS-9', DEFAULT_PASSWORD_POLICY);
    expect(r.message).toMatch(/lowercase/i);
  });

  it('requires a digit', () => {
    const r = validatePassword('NoDigitsHere', DEFAULT_PASSWORD_POLICY);
    expect(r.message).toMatch(/number/i);
  });

  it('rejects common passwords that pass every character-class rule', () => {
    // "Passw0rd1" has 9 chars, an uppercase, a lowercase, a digit — it
    // satisfies every char-class rule but is in the server blocklist.
    const r = validatePassword('Passw0rd1', DEFAULT_PASSWORD_POLICY);
    expect(r.valid).toBe(false);
    expect(r.meetsAllRules).toBe(false);
    expect(r.message).toMatch(/common/i);
  });

  it('accepts a strong password that is not in the common list', () => {
    const r = validatePassword('Stronger99!X', DEFAULT_PASSWORD_POLICY);
    expect(r.valid).toBe(true);
    expect(r.meetsAllRules).toBe(true);
    expect(r.message).toBe('');
  });

  it('does not enforce common-password block when the policy disables it', () => {
    const policy: PasswordPolicy = { ...DEFAULT_PASSWORD_POLICY, blockCommonPasswords: false };
    const r = validatePassword('Passw0rd1', policy);
    expect(r.valid).toBe(true);
  });
});

describe('describePolicy', () => {
  it('mentions every active rule on the default policy', () => {
    const text = describePolicy(DEFAULT_PASSWORD_POLICY);
    expect(text).toContain('At least 8 characters');
    expect(text).toMatch(/upper and lowercase letters/i);
    expect(text).toContain('a number');
    expect(text).toMatch(/avoid commonly-used passwords/i);
  });

  it('drops the common-password clause when the policy disables it', () => {
    const text = describePolicy({ ...DEFAULT_PASSWORD_POLICY, blockCommonPasswords: false });
    expect(text).not.toMatch(/common/i);
  });
});
