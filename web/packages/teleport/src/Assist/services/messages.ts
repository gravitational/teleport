export enum Author {
  Teleport,
  User,
}

export enum Type {
  Exec = 'exec',
  Message = 'message',
  Connect = 'connect',
}

export interface MessageContent {
  type: Type;
  value: string | string[];
}

export interface Message {
  hidden?: boolean;
  content: MessageContent[];
  author: Author;
}

interface CommandOutput {
  serverName: string;
  commandOutput: string;
}

export interface ExecOutput {
  commandOutputs: CommandOutput[];
  humanInterpretation: string;
}

export async function sendMessage(
  messages: Message[]
): Promise<MessageContent[]> {
  const res = await fetch('/api/request', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(messages),
  });

  return res.json();
}

export async function exec(contents: MessageContent[]): Promise<ExecOutput[]> {
  const res = await fetch('/api/exec', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(contents),
  });

  return res.json();
}
