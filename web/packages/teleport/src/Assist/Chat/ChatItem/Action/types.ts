export interface CommandState {
  type: 'command';
  value: string;
}

export interface NodeState {
  type: 'node';
  value: string;
}

export interface UserState {
  type: 'user';
  value: string;
}

export interface LabelState {
  type: 'label';
  value: {
    key: string;
    value: string;
  };
}

export type ActionState = CommandState | NodeState | LabelState | UserState;
