import { gql } from '@apollo/client';

// ── Task Operations ─────────────────────────────────────────────────

export const MY_TASKS_QUERY = gql`
  query MyTasks($page: Int, $pageSize: Int) {
    myTasks(page: $page, pageSize: $pageSize) {
      data {
        id type status progress result error createdAt updatedAt completedAt
      }
      total
    }
  }
`;

export const CREATE_TASK = gql`
  mutation CreateTask($input: CreateTaskInput!) {
    createTask(input: $input) {
      id type status createdAt
    }
  }
`;

export const CANCEL_TASK = gql`
  mutation CancelTask($id: ID!) {
    cancelTask(id: $id) { id status }
  }
`;
