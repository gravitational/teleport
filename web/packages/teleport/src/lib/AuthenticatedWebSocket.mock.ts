// MockAuthenticatedWebSocket is a mock implementation of the AuthenticatedWebSocket class
// that simulates a WebSocket connection for testing purposes.
export class MockAuthenticatedWebSocket {
  private _ws: WebSocket;

  send: (data: string) => void;
  close: (code?: number, reason?: string) => void;
  onopen: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
  onclose: ((event: CloseEvent) => void) | null;
  addEventListener: (
    type: string,
    listener: EventListenerOrEventListenerObject
  ) => void;
  removeEventListener: (
    type: string,
    listener: EventListenerOrEventListenerObject
  ) => void;
  dispatchEvent: (event: Event) => boolean;

  constructor(url: string) {
    // Convert relative URL to absolute WebSocket URL for testing
    const wsUrl = url.startsWith('/') ? `ws://localhost${url}` : url;

    const ws = new WebSocket(wsUrl);

    // Store the underlying WebSocket
    this._ws = ws;

    // Initialize properties
    this.onopen = null;
    this.onmessage = null;
    this.onerror = null;
    this.onclose = null;

    // Set up event handlers
    ws.onopen = event => {
      if (this.onopen) {
        this.onopen(event);
      }
    };

    ws.onmessage = event => {
      if (this.onmessage) {
        this.onmessage(event);
      }
    };

    ws.onerror = event => {
      if (this.onerror) {
        this.onerror(event);
      }
    };

    ws.onclose = event => {
      if (this.onclose) {
        this.onclose(event);
      }
    };

    // Bind methods
    this.send = data => {
      ws.send(data);
    };

    this.close = (code, reason) => {
      ws.close(code, reason);
    };

    this.addEventListener = (type, listener) =>
      ws.addEventListener(type, listener);
    this.removeEventListener = (type, listener) =>
      ws.removeEventListener(type, listener);
    this.dispatchEvent = event => ws.dispatchEvent(event);
  }

  // Getters for WebSocket properties
  get readyState() {
    return this._ws.readyState;
  }
  get url() {
    return this._ws.url;
  }
  get protocol() {
    return this._ws.protocol;
  }
  get extensions() {
    return this._ws.extensions;
  }
  get bufferedAmount() {
    return this._ws.bufferedAmount;
  }
  get binaryType() {
    return this._ws.binaryType;
  }
  set binaryType(value) {
    this._ws.binaryType = value;
  }
}
