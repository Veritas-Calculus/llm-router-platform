import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  AddOrgMemberMutation,
  AddOrgMemberMutationVariables,
  GetOrgMembersQuery,
  GetOrgMembersQueryVariables,
  RemoveOrgMemberMutation,
  RemoveOrgMemberMutationVariables,
  UpdateOrgMemberRoleMutation,
  UpdateOrgMemberRoleMutationVariables,
} from '../generated/graphql';

export const GET_ORG_MEMBERS: TypedDocumentNode<GetOrgMembersQuery, GetOrgMembersQueryVariables> = gql`
  query GetOrgMembers($orgId: ID!) {
        organizationMembers(orgId: $orgId) {
            userId
            orgId
            role
            createdAt
            user {
                id
                name
                email
                role
                isActive
                createdAt
            }
        }
    }
`;

export const ADD_ORG_MEMBER: TypedDocumentNode<AddOrgMemberMutation, AddOrgMemberMutationVariables> = gql`
  mutation AddOrgMember($orgId: ID!, $email: String!, $role: String!) {
        addOrganizationMember(orgId: $orgId, email: $email, role: $role) {
            userId
            role
        }
    }
`;

export const UPDATE_ORG_MEMBER_ROLE: TypedDocumentNode<UpdateOrgMemberRoleMutation, UpdateOrgMemberRoleMutationVariables> = gql`
  mutation UpdateOrgMemberRole($orgId: ID!, $userId: ID!, $role: String!) {
        updateOrganizationMemberRole(orgId: $orgId, userId: $userId, role: $role) {
            userId
            role
        }
    }
`;

export const REMOVE_ORG_MEMBER: TypedDocumentNode<RemoveOrgMemberMutation, RemoveOrgMemberMutationVariables> = gql`
  mutation RemoveOrgMember($orgId: ID!, $userId: ID!) {
        removeOrganizationMember(orgId: $orgId, userId: $userId)
    }
`;
