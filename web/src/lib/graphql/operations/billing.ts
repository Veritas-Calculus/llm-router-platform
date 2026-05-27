import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  ChangePlanMutation,
  ChangePlanMutationVariables,
  CreateCheckoutSessionMutation,
  CreateCheckoutSessionMutationVariables,
  CreatePlanMutation,
  CreatePlanMutationVariables,
  CreateRechargeSessionMutation,
  CreateRechargeSessionMutationVariables,
  DeleteBudgetMutation,
  DeleteBudgetMutationVariables,
  MyBillingQuery,
  MyBillingQueryVariables,
  PlansQuery,
  PlansQueryVariables,
  SetBudgetMutation,
  SetBudgetMutationVariables,
  SubscriptionQuotaQuery,
  SubscriptionQuotaQueryVariables,
  UpdatePlanMutation,
  UpdatePlanMutationVariables,
} from '../generated/graphql';

// ── Billing Operations ──────────────────────────────────────────────

export const SUBSCRIPTION_QUOTA_QUERY: TypedDocumentNode<SubscriptionQuotaQuery, SubscriptionQuotaQueryVariables> = gql`
  query SubscriptionQuota {
    mySubscription { id planId planName status currentPeriodStart currentPeriodEnd usedTokens tokenLimit quotaPercentage isQuotaExceeded }
  }
`;

export const MY_BILLING_QUERY: TypedDocumentNode<MyBillingQuery, MyBillingQueryVariables> = gql`
  query MyBilling {
    mySubscription { id planId planName status currentPeriodStart currentPeriodEnd usedTokens tokenLimit quotaPercentage isQuotaExceeded }
    myBudget { id monthlyLimitUsd alertThreshold enforceHardLimit isActive }
    myBudgetStatus { currentSpend remainingBudget percentUsed isOverBudget }
    myOrders { id orderNo amount currency status paymentMethod createdAt }
  }
`;

export const PLANS_QUERY: TypedDocumentNode<PlansQuery, PlansQueryVariables> = gql`
  query Plans {
    plans { id name description priceMonth tokenLimit rateLimit supportLevel features isActive }
  }
`;

export const SET_BUDGET: TypedDocumentNode<SetBudgetMutation, SetBudgetMutationVariables> = gql`
  mutation SetBudget($input: BudgetInput!) {
    setBudget(input: $input) { id monthlyLimitUsd alertThreshold }
  }
`;

export const DELETE_BUDGET: TypedDocumentNode<DeleteBudgetMutation, DeleteBudgetMutationVariables> = gql`
  mutation DeleteBudget {
    deleteBudget
  }
`;

export const CHANGE_PLAN: TypedDocumentNode<ChangePlanMutation, ChangePlanMutationVariables> = gql`
  mutation ChangePlan($planId: ID!) {
    changePlan(planId: $planId) { id planId planName status currentPeriodStart currentPeriodEnd }
  }
`;

export const CREATE_CHECKOUT_SESSION: TypedDocumentNode<CreateCheckoutSessionMutation, CreateCheckoutSessionMutationVariables> = gql`
  mutation CreateCheckoutSession($planId: ID!) {
    createCheckoutSession(planId: $planId) { url }
  }
`;

// ── Admin: Plans ──
export const CREATE_RECHARGE_SESSION: TypedDocumentNode<CreateRechargeSessionMutation, CreateRechargeSessionMutationVariables> = gql`
  mutation CreateRechargeSession($amount: Money!) {
    createRechargeSession(amount: $amount) { url }
  }
`;

export const CREATE_PLAN: TypedDocumentNode<CreatePlanMutation, CreatePlanMutationVariables> = gql`
  mutation CreatePlan($input: PlanInput!) {
    createPlan(input: $input) { id name description priceMonth tokenLimit rateLimit supportLevel features isActive }
  }
`;

export const UPDATE_PLAN: TypedDocumentNode<UpdatePlanMutation, UpdatePlanMutationVariables> = gql`
  mutation UpdatePlan($id: ID!, $input: PlanInput!) {
    updatePlan(id: $id, input: $input) { id name description priceMonth tokenLimit rateLimit supportLevel features isActive }
  }
`;
