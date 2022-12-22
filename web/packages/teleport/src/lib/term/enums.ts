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

// See https://developer.mozilla.org/en-US/docs/Web/API/CloseEvent/code
export enum WebsocketCloseCode {
  NORMAL = 1000,
  ABNORMAL = 1006,
}
