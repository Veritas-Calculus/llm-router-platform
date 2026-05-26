import { gql } from '@apollo/client';

// ── Auth Operations ─────────────────────────────────────────────────

export const LOGIN = gql`
  mutation Login($input: LoginInput!) {
    login(input: $input) {
      token
      user { id email name role isActive mfaEnabled emailVerified }
    }
  }
`;

export const REGISTER = gql`
  mutation Register($input: RegisterInput!) {
    register(input: $input) {
      token
      user { id email name role isActive mfaEnabled emailVerified }
    }
  }
`;

export const REFRESH_TOKEN = gql`
  mutation RefreshToken {
    refreshToken {
      token
      user { id email name role isActive mfaEnabled emailVerified }
    }
  }
`;

// ROTATE_REFRESH_TOKEN is the canonical refresh path. The refresh token is
// carried by an HttpOnly cookie so browser JavaScript never reads or persists
// it; the empty argument keeps the legacy schema signature compatible.
export const ROTATE_REFRESH_TOKEN = gql`
  mutation RotateRefreshToken {
    rotateRefreshToken(refreshToken: "") {
      token
      user { id email name role isActive mfaEnabled emailVerified }
    }
  }
`;

export const LOGOUT = gql`
  mutation Logout {
    logout
  }
`;

export const FORGOT_PASSWORD = gql`
  mutation ForgotPassword($email: String!) {
    forgotPassword(email: $email)
  }
`;

export const RESET_PASSWORD = gql`
  mutation ResetPassword($input: ResetPasswordInput!) {
    resetPassword(input: $input)
  }
`;

export const CHANGE_PASSWORD = gql`
  mutation ChangePassword($input: ChangePasswordInput!) {
    changePassword(input: $input)
  }
`;

export const UPDATE_PROFILE = gql`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input) {
      id email name role isActive mfaEnabled emailVerified
    }
  }
`;

export const ME = gql`
  query Me {
    me { id email name role isActive mfaEnabled emailVerified createdAt }
  }
`;

// MY_BALANCE is a lightweight query for refreshing user balance after a
// completed payment. We keep it separate from ME so common auth flows don't
// have to round-trip a financial field they don't render.
export const MY_BALANCE = gql`
  query MyBalance {
    me { id balance }
  }
`;

export const REGISTRATION_MODE = gql`
  query RegistrationMode {
    registrationMode {
      mode
      inviteCodeRequired
    }
  }
`;

export const CAPTCHA_CONFIG = gql`
  query CaptchaConfig {
    captchaConfig {
      enabled
      siteKey
    }
  }
`;

export const DISCOVER_SSO = gql`
  mutation DiscoverSso($email: String!) {
    discoverSso(email: $email) {
      redirectUrl
    }
  }
`;

export const VERIFY_EMAIL = gql`
  mutation VerifyEmail($token: String!) {
    verifyEmail(token: $token)
  }
`;

export const RESEND_VERIFICATION_EMAIL = gql`
  mutation ResendVerificationEmail {
    resendVerificationEmail
  }
`;

// -- MFA Operations --

export const GENERATE_MFA_SECRET = gql`
  mutation GenerateMfaSecret {
    generateMfaSecret {
      secret
      qrCodeUrl
      backupCodes
    }
  }
`;

export const VERIFY_AND_ENABLE_MFA = gql`
  mutation VerifyAndEnableMfa($code: String!) {
    verifyAndEnableMfa(code: $code)
  }
`;

export const DISABLE_MFA = gql`
  mutation DisableMfa($code: String!) {
    disableMfa(code: $code)
  }
`;
