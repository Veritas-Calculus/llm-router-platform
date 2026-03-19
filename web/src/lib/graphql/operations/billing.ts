import { gql } from '@apollo/client';

// ── Billing Operations ──────────────────────────────────────────────

export const MY_BILLING_QUERY = gql`
  query MyBilling {
    mySubscription { id planId planName status currentPeriodStart currentPeriodEnd }
    myBudget { id monthlyLimit alertThreshold currentSpend }
    myBudgetStatus { used limit percentage isOverBudget }
    myOrders { id amount currency status description createdAt }
  }
`;

export const PLANS_QUERY = gql`
  query Plans {
    plans { id name description price currency interval features isActive }
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

export const CREATE_CHECKOUT_SESSION = gql`
  mutation CreateCheckoutSession($planId: ID!) {
    createCheckoutSession(planId: $planId) { sessionId url }
  }
`;

export const CREATE_RECHARGE_SESSION = gql`
  mutation CreateRechargeSession($amount: Float!) {
    createRechargeSession(amount: $amount) { sessionId url }
  }
`;

// ── Admin: Plans ──
export const CREATE_PLAN = gql`
  mutation CreatePlan($input: PlanInput!) {
    createPlan(input: $input) { id name price isActive }
  }
`;

export const UPDATE_PLAN = gql`
  mutation UpdatePlan($id: ID!, $input: PlanInput!) {
    updatePlan(id: $id, input: $input) { id name price isActive }
  }
`;
