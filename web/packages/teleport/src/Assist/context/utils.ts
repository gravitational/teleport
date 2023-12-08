/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
    case ServerMessageType.CommandResultSummary:
    case ServerMessageType.Error:
    case ServerMessageType.AccessRequests:
    case ServerMessageType.AccessRequest:
    case ServerMessageType.AccessRequestCreated:
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
