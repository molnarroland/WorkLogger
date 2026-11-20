export type TaskState = 'running' | 'done' | 'failed' | 'cancelled';

/** One piece of work this window started (a job, a send, a Tempo read-back) and how it ended. */
export interface Task {
  id: number;
  label: string;
  state: TaskState;
  /** The result or the error, in the UI language. */
  message: string;
  started: string;
  ended?: string;
}
