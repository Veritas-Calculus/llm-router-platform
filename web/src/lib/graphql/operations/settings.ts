import { gql } from '@apollo/client';

// ── Settings Operations ─────────────────────────────────────────────

export const SYSTEM_SETTINGS_QUERY = gql`
  query SystemSettings {
    systemSettings {
      registrationMode
      defaultTokenLimit
      defaultBudgetUsd
      site
      security
      defaults
      email
      backup
      payment
      oauth
    }
    inviteCodes { id code createdBy maxUses useCount expiresAt isActive createdAt }
  }
`;

export const UPDATE_SYSTEM_SETTINGS = gql`
  mutation UpdateSystemSettings($input: SystemSettingsInput!) {
    updateSystemSettings(input: $input) {
      registrationMode
      site
      security
      defaults
      email
      backup
      payment
      oauth
    }
  }
`;

export const CREATE_INVITE_CODE = gql`
  mutation CreateInviteCode($input: InviteCodeInput!) {
    createInviteCode(input: $input) {
      id code maxUses expiresAt isActive createdAt
    }
  }
`;

export const SEND_TEST_EMAIL = gql`
  mutation SendTestEmail($to: String!) {
    sendTestEmail(to: $to)
  }
`;

export const TRIGGER_BACKUP = gql`
  mutation TriggerBackup {
    triggerBackup
  }
`;

// ── Settings Page (User) ──

export const MY_SETTINGS_QUERY = gql`
  query MySettings {
    me { id email name role createdAt }
    mySubscription { id planName status currentPeriodEnd }
    myBudgetStatus { used limit percentage isOverBudget }
    myAnomalyDetection { hasAnomaly message }
  }
`;
