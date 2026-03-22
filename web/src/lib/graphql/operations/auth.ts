import { gql } from '@apollo/client';

// ── Auth Operations ─────────────────────────────────────────────────

export const LOGIN = gql`
  mutation Login($input: LoginInput!) {
    login(input: $input) {
      token
      refreshToken
      user { id email name role isActive mfaEnabled }
    }
  }
`;

export const REGISTER = gql`
  mutation Register($input: RegisterInput!) {
    register(input: $input) {
      token
      refreshToken
      user { id email name role isActive mfaEnabled }
    }
  }
`;

export const REFRESH_TOKEN = gql`
  mutation RefreshToken {
    refreshToken {
      token
      refreshToken
      user { id email name role isActive mfaEnabled }
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
      id email name role isActive mfaEnabled
    }
  }
`;

export const ME = gql`
  query Me {
    me { id email name role isActive mfaEnabled createdAt }
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
