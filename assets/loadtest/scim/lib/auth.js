import { RequestType, HttpMethod, HttpStatus } from './scim.js';

/**
 * TokenExchange implements token exchange based on OAuth 2.0 client credential grant.
 * Each k6 VU maintains its own token, for example, VUS=20 will have 20 token exchanged
 * for each VU.
 */
export class TokenExchange {
  constructor({ tokenUrl, clientId, clientSecret }, transport, now = Date.now) {
    this.tokenUrl = tokenUrl;
    this.clientId = clientId;
    this.clientSecret = clientSecret;
    this.transport = transport;
    this.now = now;
    this.cached = null;
  }

  // token returns the cached token, requesting a new one when it is missing or
  // due for refresh.
  token() {
    if (!this.cached || this.now() >= this.cached.refreshAt) {
      this.cached = this.#requestToken();
    }
    return this.cached.value;
  }

  #requestToken() {
    const grantType = 'client_credentials';
    const form = {
      grant_type: grantType,
      client_id: this.clientId,
      client_secret: this.clientSecret,
    };

    const res = this.transport.request(HttpMethod.POST, this.tokenUrl, form, {
      requestType: RequestType.TOKEN,
    });
    if (res.status !== HttpStatus.Ok) {
      throw new Error(
        `token request to ${this.tokenUrl} failed: status=${res.status} ${res.error || res.body}`
      );
    }

    const value = res.json('access_token');
    if (!value) {
      throw new Error(`token response has no access_token: ${res.body}`);
    }

    const expiresIn = res.json('expires_in');
    // Refresh this long before the token expires (tokens are valid for 1h).
    const refreshMargin = 300;
    return {
      value,
      refreshAt: this.now() + (expiresIn - refreshMargin) * 1000,
    };
  }
}
