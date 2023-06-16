/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Author, ServerMessageType } from 'teleport/Assist/types';

import type {
  ConversationMessage,
  ExecuteRemoteCommandPayload,
  ResolvedServerMessage,
} from 'teleport/Assist/types';

function getMessageTypeAuthor(type: string) {
  switch (type) {
    case ServerMessageType.User:
      return Author.User;

    case ServerMessageType.Assist:
    case ServerMessageType.AssistThought:
    case ServerMessageType.Command:
    case ServerMessageType.CommandResult:
    case ServerMessageType.CommandResultStream:
      return Author.Teleport;
  }
}

function findConsecutiveMessagesFromSameAuthor(
  messages: ResolvedServerMessage[],
  index: number,
  author: Author
) {
  let i = index;

  const consecutiveMessagesFromSameAuthor: ResolvedServerMessage[] = [];

  while (i < messages.length) {
    if (getMessageTypeAuthor(messages[i].type) !== author) {
      break;
    }

    consecutiveMessagesFromSameAuthor.push(messages[i]);

    i++;
  }

  return { nextIndex: i, consecutiveMessagesFromSameAuthor };
}

export function convertServerMessages(
  messages: ResolvedServerMessage[]
): ConversationMessage[] {
  const conversationMessages: ConversationMessage[] = [];

  let i = 0;

  while (i < messages.length) {
    const message = messages[i];
    const author = getMessageTypeAuthor(message.type);

    const { nextIndex, consecutiveMessagesFromSameAuthor } =
      findConsecutiveMessagesFromSameAuthor(messages, i, author);

    const timestamp =
      consecutiveMessagesFromSameAuthor[
        consecutiveMessagesFromSameAuthor.length - 1
      ].created;

    const conversationMessage: ConversationMessage = {
      streaming: false,
      entries: consecutiveMessagesFromSameAuthor,
      author,
      timestamp,
    };

    conversationMessages.push(conversationMessage);

    i = nextIndex;
  }

  return conversationMessages;
}

export function convertPayloadToQuery(payload: ExecuteRemoteCommandPayload) {
  let query = '';

  if (payload.nodes?.length) {
    query += payload.nodes.map(node => `name == "${node}"`).join(' || ');
  }

  if (payload.labels) {
    if (payload.nodes?.length) {
      query += ' || ';
    }

    query += payload.labels
      .map(label => `labels["${label.key}"] == "${label.value}"`)
      .join(' || ');
  }

  return query;
}

const ROOT_LOGINS = ['root', 'ec2-user', 'ubuntu', 'admin', 'centos'];

export function sortLoginsWithRootLoginsLast(logins: string[]): string[] {
  return logins.sort((a, b) => {
    if (ROOT_LOGINS.includes(a) && !ROOT_LOGINS.includes(b)) {
      return 1;
    }

    if (!ROOT_LOGINS.includes(a) && ROOT_LOGINS.includes(b)) {
      return -1;
    }

    return a.localeCompare(b);
  });
}

export function findIntersection<T>(elements: T[][]): T[] {
  if (elements.length == 0) {
    return [];
  }

  if (elements.length == 1) {
    return elements[0];
  }

  const intersectSets = (a: Set<T>, b: Set<T>) => {
    const c = new Set<T>();
    a.forEach(v => b.has(v) && c.add(v));
    return c;
  };

  return [...elements.map(e => new Set(e)).reduce(intersectSets)];
}
