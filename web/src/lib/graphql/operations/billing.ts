import { gql } from '@apollo/client';

// ── Billing Operations ──────────────────────────────────────────────

export const SUBSCRIPTION_QUOTA_QUERY = gql`
  query SubscriptionQuota {
    mySubscription { id planId planName status currentPeriodStart currentPeriodEnd usedTokens tokenLimit quotaPercentage isQuotaExceeded }
  }
`;

export const MY_BILLING_QUERY = gql`
  query MyBilling {
    mySubscription { id planId planName status currentPeriodStart currentPeriodEnd usedTokens tokenLimit quotaPercentage isQuotaExceeded }
    myBudget { id monthlyLimitUsd alertThreshold enforceHardLimit isActive }
    myBudgetStatus { currentSpend remainingBudget percentUsed isOverBudget }
    myOrders { id orderNo amount currency status paymentMethod createdAt }
  }
`;

export const PLANS_QUERY = gql`
  query Plans {
    plans { id name description priceMonth tokenLimit rateLimit supportLevel features isActive }
  }
`;

export const SET_BUDGET = gql`
  mutation SetBudget($input: BudgetInput!) {
    setBudget(input: $input) { id monthlyLimit alertThreshold }
  }
`;

export const DELETE_BUDGET = gql`
  mutation DeleteBudget {
    deleteBudget
  }
`;

export const CHANGE_PLAN = gql`
  mutation ChangePlan($planId: ID!) {
    changePlan(planId: $planId) { id planId planName status currentPeriodStart currentPeriodEnd }
  }
`;

// ── Admin: Plans ──
export const CREATE_PLAN = gql`
  mutation CreatePlan($input: PlanInput!) {
    createPlan(input: $input) { id name priceMonth isActive }
  }
`;

export const UPDATE_PLAN = gql`
  mutation UpdatePlan($id: ID!, $input: PlanInput!) {
    updatePlan(id: $id, input: $input) { id name priceMonth isActive }
  }
`;
