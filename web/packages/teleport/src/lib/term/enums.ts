/*
Copyright 2019-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

export enum EventType {
  START = 'session.start',
  JOIN = 'session.join',
  END = 'session.end',
  PRINT = 'print',
  RESIZE = 'resize',
}

export enum TermEvent {
  RESIZE = 'terminal.resize',
  CLOSE = 'terminal.close',
  RESET = 'terminal.reset',
  SESSION = 'terminal.new_session',
  DATA = 'terminal.data',
  CONN_CLOSE = 'connection.close',
  WEBAUTHN_CHALLENGE = 'terminal.webauthn',
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
