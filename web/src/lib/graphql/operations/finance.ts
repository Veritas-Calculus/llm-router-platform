import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  AdminFinancialDashboardQuery,
  AdminFinancialDashboardQueryVariables,
} from '../generated/graphql';

export const ADMIN_FINANCIAL_DASHBOARD_QUERY: TypedDocumentNode<AdminFinancialDashboardQuery, AdminFinancialDashboardQueryVariables> = gql`
  query AdminFinancialDashboard($days: Int) {
    adminFinancialDashboard(days: $days) {
      periodStart
      periodEnd
      cashRevenue
      netCashRevenue
      subscriptionRevenue
      topUpRevenue
      usageRevenue
      providerCost
      grossProfit
      grossMargin
      refundAmount
      creditGrants
      outstandingBalance
      paidOrders
      activeSubscriptions
      payingCustomers
      arpu
      daily {
        date
        cashRevenue
        usageRevenue
        providerCost
        grossProfit
        orders
        requests
      }
      paymentBreakdown {
        name
        amount
        count
      }
      providerBreakdown {
        providerName
        requests
        usageRevenue
        providerCost
        grossProfit
        grossMargin
      }
    }
  }
`;
