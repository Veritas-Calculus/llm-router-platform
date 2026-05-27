import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CreateRoutingRuleMutation,
  CreateRoutingRuleMutationVariables,
  DeleteRoutingRuleMutation,
  DeleteRoutingRuleMutationVariables,
  GetRoutingRulesQuery,
  GetRoutingRulesQueryVariables,
  UpdateRoutingRuleMutation,
  UpdateRoutingRuleMutationVariables,
} from '../generated/graphql';

export const GET_ROUTING_RULES: TypedDocumentNode<GetRoutingRulesQuery, GetRoutingRulesQueryVariables> = gql`
  query GetRoutingRules($page: Int, $pageSize: Int) {
    routingRules(page: $page, pageSize: $pageSize) {
      data {
        id
        name
        description
        modelPattern
        targetProviderId
        fallbackProviderId
        priority
        isEnabled
        createdAt
        updatedAt
        targetProvider {
          id
          name
          isActive
        }
        fallbackProvider {
          id
          name
          isActive
        }
      }
      total
      page
      pageSize
    }
  }
`;

export const CREATE_ROUTING_RULE: TypedDocumentNode<CreateRoutingRuleMutation, CreateRoutingRuleMutationVariables> = gql`
  mutation CreateRoutingRule($input: CreateRoutingRuleInput!) {
    createRoutingRule(input: $input) {
      id
      name
      description
      modelPattern
      targetProviderId
      fallbackProviderId
      priority
      isEnabled
    }
  }
`;

export const UPDATE_ROUTING_RULE: TypedDocumentNode<UpdateRoutingRuleMutation, UpdateRoutingRuleMutationVariables> = gql`
  mutation UpdateRoutingRule($id: ID!, $input: UpdateRoutingRuleInput!) {
    updateRoutingRule(id: $id, input: $input) {
      id
      name
      description
      modelPattern
      targetProviderId
      fallbackProviderId
      priority
      isEnabled
    }
  }
`;

export const DELETE_ROUTING_RULE: TypedDocumentNode<DeleteRoutingRuleMutation, DeleteRoutingRuleMutationVariables> = gql`
  mutation DeleteRoutingRule($id: ID!) {
    deleteRoutingRule(id: $id)
  }
`;
