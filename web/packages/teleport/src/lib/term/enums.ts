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

export enum EventType {
  START = 'session.start',
  JOIN = 'session.join',
  END = 'session.end',
  EXEC = 'exec',
  PRINT = 'print',
  RESIZE = 'resize',
  FILE_TRANSFER_REQUEST = 'file_transfer_request',
  FILE_TRANSFER_DECISION = 'file_transfer_decision',
  FILE_TRANSFER_REQUEST_APPROVE = 'file_transfer_request_approve',
  FILE_TRANSFER_REQUEST_DENY = 'file_transfer_request_deny',

  CHAT_MESSAGE = 'chat_message',
}

export enum TermEvent {
  RESIZE = 'terminal.resize',
  CLOSE = 'terminal.close',
  RESET = 'terminal.reset',
  SESSION = 'terminal.new_session',
  SESSION_STATUS = 'terminal.session_status',
  DATA = 'terminal.data',
  CONN_CLOSE = 'connection.close',
  MFA_CHALLENGE = 'terminal.webauthn',
  LATENCY = 'terminal.latency',
  KUBE_EXEC = 'terminal.kube_exec',
}

// Websocket connection close codes.
// If unset, the browser will automtically set the code to a standard value.
// If specified, the value must be 1000 or a custom code in the range 3000-4999.
//
// See:
// - https://developer.mozilla.org/en-US/docs/Web/API/WebSocket/close
// - https://developer.mozilla.org/en-US/docs/Web/API/CloseEvent/code
export enum WebsocketCloseCode {
  NORMAL = 1000,
}
