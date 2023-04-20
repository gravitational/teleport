export interface CommandState {
  type: 'command';
  value: string;
}

export interface QueryState {
  type: 'query';
  value: string;
}

export interface UserState {
  type: 'user';
  value: string;
}

export type ActionState = CommandState | QueryState | UserState;
