import { gql, type TypedDocumentNode } from '@apollo/client';
import type {
  CancelTaskMutation,
  CancelTaskMutationVariables,
  CreateTaskMutation,
  CreateTaskMutationVariables,
  MyTasksQuery,
  MyTasksQueryVariables,
} from '../generated/graphql';

// ── Task Operations ─────────────────────────────────────────────────

export const MY_TASKS_QUERY: TypedDocumentNode<MyTasksQuery, MyTasksQueryVariables> = gql`
  query MyTasks($page: Int, $pageSize: Int) {
    myTasks(page: $page, pageSize: $pageSize) {
      data {
        id type status progress result error createdAt startedAt completedAt
      }
      total
    }
  }
`;

export const CREATE_TASK: TypedDocumentNode<CreateTaskMutation, CreateTaskMutationVariables> = gql`
  mutation CreateTask($input: CreateTaskInput!) {
    createTask(input: $input) {
      id type status createdAt
    }
  }
`;

export const CANCEL_TASK: TypedDocumentNode<CancelTaskMutation, CancelTaskMutationVariables> = gql`
  mutation CancelTask($id: ID!) {
    cancelTask(id: $id) { id status }
  }
`;
