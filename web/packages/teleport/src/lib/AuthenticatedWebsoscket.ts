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

import { getAccessToken } from 'teleport/services/api';
import { WebsocketStatus } from 'teleport/types';

export class AuthenticatedWebSocket {
  ws: WebSocket | undefined;

  onopenAfterAuth: (ev: Event) => void | undefined;
  onmessageAfterAuth: (ev: MessageEvent) => void | undefined;
  oncloseAfterAuth: (ev: CloseEvent) => void | undefined;

  private authenticated: boolean;

  constructor(
    socketAddr: string,
    onopen: (ev: Event) => void | null,
    onmessage: (ev: MessageEvent) => void | null,
    onerror: (ev: Event) => void | null,
    onclose: (ev: CloseEvent) => void | null
  ) {
    this.onopen = this.onopen.bind(this);
    this.onmessage = this.onmessage.bind(this);
    this.onclose = this.onclose.bind(this);

    this.authenticated = false;
    this.onmessageAfterAuth = onmessage;
    this.onopenAfterAuth = onopen;
    this.oncloseAfterAuth = onclose;

    this.ws = new WebSocket(socketAddr);
    this.ws.binaryType = 'arraybuffer';

    this.ws.onopen = this.onopen;
    this.ws.onmessage = this.onmessage;
    this.ws.onerror = onerror;
    this.ws.onclose = this.onclose;
  }

  onopen(): void {
    this.ws.send(JSON.stringify({ token: getAccessToken() }));
  }

  onmessage(ev: MessageEvent): void {
    if (!this.authenticated) {
      const authResponse = JSON.parse(ev.data) as WebsocketStatus;
      if (authResponse.type != 'create_session_response') {
        this.ws.close();
        console.log('invalid auth response type: ' + authResponse.message);
        return;
      }

      if (authResponse.status == 'error') {
        this.ws.close();
        console.log(
          'auth error connecting to websocket: ' + authResponse.message
        );
        return;
      }
      this.authenticated = true;

      if (this.onopenAfterAuth) {
        this.onopenAfterAuth(ev);
      }

      return;
    }

    if (this.onmessageAfterAuth) {
      this.onmessageAfterAuth(ev);
    }
  }

  onclose(ev: CloseEvent): void {
    if (this.oncloseAfterAuth) {
      this.oncloseAfterAuth(ev);
    }
    this.authenticated = false;
    this.ws = null;
  }

  send(data: string | ArrayBufferLike | Blob | ArrayBufferView): void {
    this.ws.send(data);
  }

  close(code?: number, reason?: string): void {
    this.ws.close(code, reason);
  }
}
