/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

/**
 * `AuthenticatedWebSocket` is a drop-in replacement for
 * the `WebSocket` class that handles Teleport's websocket
 * authentication process.
 */
export class AuthenticatedWebSocket extends WebSocket {
  private authenticated: boolean = false;
  private openListeners: ((this: WebSocket, ev: Event) => any)[] = [];
  private onopenInternal: ((this: WebSocket, ev: Event) => any) | null = null;
  private messageListeners: ((this: WebSocket, ev: MessageEvent) => any)[] = [];
  private onmessageInternal:
    | ((this: WebSocket, ev: MessageEvent) => any)
    | null = null;
  private oncloseListeners: ((this: WebSocket, ev: CloseEvent) => any)[] = [];
  private oncloseInternal: ((this: WebSocket, ev: CloseEvent) => any) | null =
    null;
  private onerrorListeners: ((this: WebSocket, ev: Event) => any)[] = [];
  private onerrorInternal: ((this: WebSocket, ev: Event) => any) | null = null;
  private binaryTypeInternal: BinaryType = 'blob'; // Default binaryType
  private onopenEvent: Event | null = null;

  constructor(url: string | URL, protocols?: string | string[]) {
    super(url, protocols);
    // Set the binaryType to 'arraybuffer' to handle the authentication process.
    super.binaryType = 'arraybuffer';

    // The open event listener should immediately send the authentication token
    super.onopen = (onopenEvent: Event) => {
      super.send(JSON.stringify({ token: getAccessToken() }));
      // Don't call the user defined onopen messages yet, wait for the authentication response.
      this.onopenEvent = onopenEvent;
    };

    // The message event listener should handle the authentication response,
    // and if it succeeds, set the binaryType to the user-defined value and
    // trigger any user-added open listeners.
    super.onmessage = (ev: MessageEvent) => {
      // If not yet authenticated, handle the authentication response.
      if (!this.authenticated) {
        // Parse the message as a WebsocketStatus.
        let authResponse: WebsocketStatus;
        try {
          authResponse = JSON.parse(ev.data) as WebsocketStatus;
        } catch (e) {
          this.triggerError('Error parsing JSON from websocket message: ' + e);
          return;
        }

        // Validate the WebsocketStatus.
        if (
          !authResponse.type ||
          !authResponse.status ||
          !(authResponse.type === 'create_session_response') ||
          !(authResponse.status === 'ok' || authResponse.status === 'error')
        ) {
          this.triggerError(
            'Invalid auth response: ' + JSON.stringify(authResponse)
          );
          return;
        }

        // Authentication succeeded.
        if (authResponse.status === 'ok') {
          this.authenticated = true;
          // Set the binaryType to the value set by the user (or back to the default 'blob').
          super.binaryType = this.binaryTypeInternal;
          // Now that authentication is complete, trigger any user-added open listeners
          // with the original onopen event.
          this.openListeners.forEach(listener =>
            listener.call(this, this.onopenEvent)
          );
          this.onopenInternal?.call(this, this.onopenEvent);
          return;
        } else {
          // Authentication failed, authResponse.status === 'error'.
          this.triggerError(
            'auth error connecting to websocket: ' + authResponse.message
          );
          return;
        }
      } else {
        // If authenticated, pass messages to user-added listeners.
        this.messageListeners.forEach(listener => {
          listener.call(this, ev);
        });
        this.onmessageInternal?.call(this, ev);
      }
    };

    // Set the 'close' event for cleanup.
    super.onclose = (ev: CloseEvent) => {
      // Trigger any user-added close listeners
      this.oncloseListeners.forEach(listener => listener.call(this, ev));
      this.oncloseInternal?.call(this, ev);
      this.authenticated = false;
    };

    // Set the 'error' event for cleanup.
    super.onerror = (ev: Event) => {
      // Trigger any user-added error listeners
      this.onerrorListeners.forEach(listener => listener.call(this, ev));
      this.onerrorInternal?.call(this, ev);
      this.authenticated = false;
    };
  }

  // Authenticated send
  override send(data: string | ArrayBufferLike | Blob | ArrayBufferView): void {
    if (!this.authenticated) {
      // This should be unreachable, but just in case.
      this.triggerError(
        'Cannot send data before authentication is complete. Data: ' + data
      );
      return;
    }
    super.send(data);
  }

  // Override addEventListener to intercept these listeners and store them in
  // our appropriate arrays. They are called in the appropriate places in the
  // `onopen`, `onmessage`, `onclose`, and `onerror` methods set in the constructor.
  override addEventListener<K extends keyof WebSocketEventMap>(
    type: K,
    listener: (this: WebSocket, ev: WebSocketEventMap[K]) => any
  ): void {
    if (type === 'open') {
      this.openListeners.push(
        listener as (this: WebSocket, ev: WebSocketEventMap['open']) => any
      );
    } else if (type === 'message') {
      this.messageListeners.push(
        listener as (this: WebSocket, ev: WebSocketEventMap['message']) => any
      );
    } else if (type === 'close') {
      this.oncloseListeners.push(
        listener as (this: WebSocket, ev: WebSocketEventMap['close']) => any
      );
    } else if (type === 'error') {
      this.onerrorListeners.push(
        listener as (this: WebSocket, ev: WebSocketEventMap['error']) => any
      );
    } else {
      // This should be unreachable, but just in case.
      super.addEventListener(type, listener);
    }
  }

  // Override the onopen, onmessage, onclose, and onerror properties to store the user-defined
  // listeners in the appropriate internal properties. These are called in the appropriate places
  // in the `onopen`, `onmessage`, `onclose`, and `onerror` methods set in the constructor.

  override set onopen(listener: (this: WebSocket, ev: Event) => any | null) {
    this.onopenInternal = listener;
  }

  override get onopen(): ((this: WebSocket, ev: Event) => any) | null {
    return this.onopenInternal;
  }

  override set onmessage(
    listener: ((this: WebSocket, ev: MessageEvent) => any) | null
  ) {
    this.onmessageInternal = listener;
  }

  override get onmessage():
    | ((this: WebSocket, ev: MessageEvent) => any)
    | null {
    return this.onmessageInternal;
  }

  override set onclose(
    listener: ((this: WebSocket, ev: CloseEvent) => any) | null
  ) {
    this.oncloseInternal = listener;
  }

  override get onclose(): ((this: WebSocket, ev: CloseEvent) => any) | null {
    return this.oncloseInternal;
  }

  override set onerror(listener: ((this: WebSocket, ev: Event) => any) | null) {
    this.onerrorInternal = listener;
  }

  override get onerror(): ((this: WebSocket, ev: Event) => any) | null {
    return this.onerrorInternal;
  }

  // Override the binaryType property to store the user-defined binaryType in the appropriate internal property.
  // This is because we need to set the binaryType to 'arraybuffer' for the authentication process (see constructor),
  // and only then can we set it to the user-defined value.
  override set binaryType(binaryType: BinaryType) {
    if (this.authenticated) {
      super.binaryType = binaryType;
      return;
    }

    this.binaryTypeInternal = binaryType;
  }

  override get binaryType(): BinaryType {
    return this.binaryTypeInternal;
  }

  // Override removeEventListener to support listeners removal for 'open', 'message', and 'close' events
  override removeEventListener<K extends keyof WebSocketEventMap>(
    type: K,
    listener: (this: WebSocket, ev: WebSocketEventMap[K]) => any
  ): void {
    if (type === 'open') {
      const index = this.openListeners.indexOf(
        listener as (this: WebSocket, ev: WebSocketEventMap['open']) => any
      );
      if (index !== -1) {
        this.openListeners.splice(index, 1);
      }
    } else if (type === 'message') {
      const index = this.messageListeners.indexOf(
        listener as (this: WebSocket, ev: WebSocketEventMap['message']) => any
      );
      if (index !== -1) {
        this.messageListeners.splice(index, 1);
      }
    } else if (type === 'close') {
      const index = this.oncloseListeners.indexOf(
        listener as (this: WebSocket, ev: WebSocketEventMap['close']) => any
      );
      if (index !== -1) {
        this.oncloseListeners.splice(index, 1);
      }
    } else if (type === 'error') {
      const index = this.onerrorListeners.indexOf(
        listener as (this: WebSocket, ev: WebSocketEventMap['error']) => any
      );
      if (index !== -1) {
        this.onerrorListeners.splice(index, 1);
      }
    } else {
      // This should be unreachable, but just in case.
      super.removeEventListener(
        type,
        listener as EventListenerOrEventListenerObject
      );
    }
  }

  // Method to manually trigger an error event.
  private triggerError(errorMessage: string): void {
    const errorEvent = new ErrorEvent('error', {
      error: new Error(errorMessage),
      message: errorMessage,
    });

    // Dispatch the event to trigger all listeners attached for 'error' events.
    this.dispatchEvent(errorEvent);
  }
}
