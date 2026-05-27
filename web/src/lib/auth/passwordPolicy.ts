// passwordPolicy.ts — the single source of truth for password validation
// on the frontend. Mirrors server/internal/service/user/user.go's
// ValidatePassword exactly; if you change one, change the other.
//
// We default-init from a hard-coded constant so the signup form has a
// sensible policy to render even if the public passwordPolicy GraphQL
// query hasn't returned yet (slow network, cold cache). Once Apollo hands
// back the server-issued policy we replace the defaults in the component
// state — that's how the M-09 fix avoids re-shipping "Minimum 6 characters"
// in production.
//
// Post-audit P0-2 added the blockCommonPasswords flag so the client mirrors
// the same six rules the server enforces. Without this flag the GraphQL
// passwordPolicy query was lying about what would actually pass validation
// ("Passw0rd1" met all five advertised rules but was rejected as common).

export interface PasswordPolicy {
  minLength: number;
  requireLetter: boolean;
  requireDigit: boolean;
  requireUpper: boolean;
  requireLower: boolean;
  blockCommonPasswords: boolean;
}

// keep in sync with server/internal/service/user/user.go::ValidatePassword
export const DEFAULT_PASSWORD_POLICY: PasswordPolicy = {
  minLength: 8,
  requireLetter: true,
  requireDigit: true,
  requireUpper: true,
  requireLower: true,
  blockCommonPasswords: true,
};

// COMMON_PASSWORDS mirrors server/internal/service/user/user.go's
// `commonPasswords` map. The server is the authority; this client list
// exists only so the signup form can pre-empt a round-trip that would
// otherwise reject the user's first attempt. Lower-cased for matching.
const COMMON_PASSWORDS: ReadonlySet<string> = new Set([
  'password1', 'password12', 'password123',
  'qwerty123', 'qwertyui', 'qwerty1234',
  'abc12345', 'abcd1234', 'abcdef12',
  'welcome1', 'letmein1', 'trustno1',
  'iloveyou1', 'sunshine1', 'princess1',
  'football1', 'baseball1', 'dragon123',
  'master123', 'monkey123', 'shadow123',
  'michael1', 'jennifer1', 'charlie1',
  'admin123', 'login123', 'welcome123',
  'passw0rd1', 'p@ssword1', 'p@ssw0rd1',
  'changeme1', 'test1234', 'guest1234',
  '12345678a', '1234567890a', '123456789a',
  // Server stores these mixed-case but we compare lower-cased.
  'superman1', 'computer1', 'starwars1',
]);

export interface PasswordValidationResult {
  valid: boolean;
  /** Short, user-facing message. Empty when valid. */
  message: string;
  /** True once the password meets every rule. */
  meetsAllRules: boolean;
}

// validatePassword returns the first rule the password violates, or an
// empty string when every rule passes. Order matches the server so users
// see the same error the server would emit.
export function validatePassword(password: string, policy: PasswordPolicy = DEFAULT_PASSWORD_POLICY): PasswordValidationResult {
  if (password.length === 0) {
    return { valid: false, message: '', meetsAllRules: false };
  }
  if (password.length < policy.minLength) {
    return {
      valid: false,
      message: `At least ${policy.minLength} characters.`,
      meetsAllRules: false,
    };
  }
  if (policy.requireUpper && !/[A-Z]/.test(password)) {
    return { valid: false, message: 'Add an uppercase letter.', meetsAllRules: false };
  }
  if (policy.requireLower && !/[a-z]/.test(password)) {
    return { valid: false, message: 'Add a lowercase letter.', meetsAllRules: false };
  }
  if (policy.requireDigit && !/\d/.test(password)) {
    return { valid: false, message: 'Add a number.', meetsAllRules: false };
  }
  // requireLetter is a superset of requireUpper || requireLower; only
  // enforced when those two are off (matches the server contract).
  if (policy.requireLetter && !policy.requireUpper && !policy.requireLower && !/[A-Za-z]/.test(password)) {
    return { valid: false, message: 'Add a letter.', meetsAllRules: false };
  }
  // Common-password block — runs LAST so the user sees the more specific
  // character-class hints first. Matches the server's ValidatePassword
  // ordering exactly (post-audit P0-2).
  if (policy.blockCommonPasswords && COMMON_PASSWORDS.has(password.toLowerCase())) {
    return {
      valid: false,
      message: 'This password is too common. Try something less guessable.',
      meetsAllRules: false,
    };
  }
  return { valid: true, message: '', meetsAllRules: true };
}

// describePolicy returns the human-readable hint shown below the password
// field. Keep concise — the field is small.
export function describePolicy(policy: PasswordPolicy = DEFAULT_PASSWORD_POLICY): string {
  const parts: string[] = [`At least ${policy.minLength} characters`];
  if (policy.requireUpper && policy.requireLower) {
    parts.push('upper and lowercase letters');
  } else if (policy.requireLetter) {
    parts.push('a letter');
  }
  if (policy.requireDigit) {
    parts.push('a number');
  }
  let hint = parts.join(', ') + '.';
  if (policy.blockCommonPasswords) {
    // Trailing clause so the screen-reader cadence reads naturally.
    hint += ' Avoid commonly-used passwords.';
  }
  return hint;
}
