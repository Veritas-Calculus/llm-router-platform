import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CreateInviteCodeMutation,
  CreateInviteCodeMutationVariables,
  MySettingsQuery,
  MySettingsQueryVariables,
  SendTestEmailMutation,
  SendTestEmailMutationVariables,
  SiteConfigQuery,
  SiteConfigQueryVariables,
  SystemSettingsQuery,
  SystemSettingsQueryVariables,
  TriggerBackupMutation,
  TriggerBackupMutationVariables,
  UpdateSystemSettingsMutation,
  UpdateSystemSettingsMutationVariables,
} from '../generated/graphql';

// ── Settings Operations ─────────────────────────────────────────────

export const SYSTEM_SETTINGS_QUERY: TypedDocumentNode<SystemSettingsQuery, SystemSettingsQueryVariables> = gql`
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
      captcha
    }
    inviteCodes { id code createdBy maxUses useCount expiresAt isActive createdAt }
  }
`;

export const UPDATE_SYSTEM_SETTINGS: TypedDocumentNode<UpdateSystemSettingsMutation, UpdateSystemSettingsMutationVariables> = gql`
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
      captcha
    }
  }
`;

export const CREATE_INVITE_CODE: TypedDocumentNode<CreateInviteCodeMutation, CreateInviteCodeMutationVariables> = gql`
  mutation CreateInviteCode($input: InviteCodeInput!) {
    createInviteCode(input: $input) {
      id code maxUses expiresAt isActive createdAt
    }
  }
`;

export const SEND_TEST_EMAIL: TypedDocumentNode<SendTestEmailMutation, SendTestEmailMutationVariables> = gql`
  mutation SendTestEmail($to: String!) {
    sendTestEmail(to: $to)
  }
`;

export const TRIGGER_BACKUP: TypedDocumentNode<TriggerBackupMutation, TriggerBackupMutationVariables> = gql`
  mutation TriggerBackup {
    triggerBackup
  }
`;

// ── Settings Page (User) ──

export const MY_SETTINGS_QUERY: TypedDocumentNode<MySettingsQuery, MySettingsQueryVariables> = gql`
  query MySettings {
    me { id email name role createdAt }
    mySubscription { id planName status currentPeriodEnd }
    myBudgetStatus { currentSpend remainingBudget percentUsed isOverBudget }
    myAnomalyDetection { hasAnomaly message }
  }
`;

export const SITE_CONFIG_QUERY: TypedDocumentNode<SiteConfigQuery, SiteConfigQueryVariables> = gql`
  query SiteConfig {
    siteConfig {
      siteName
      subtitle
      logoUrl
      faviconUrl
    }
  }
`;
