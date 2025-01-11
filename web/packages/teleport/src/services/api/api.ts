import 'whatwg-fetch';

import websession from 'teleport/services/websession';
import TeleportContext from 'teleport/teleportContext';

import { AuthService, MfaChallengeScope } from '../auth/auth';
import { MfaChallengeResponse } from '../mfa';
import { storageService } from '../storageService';
import parseError, { ApiError, parseProxyVersion } from './parseError';

export const MFA_HEADER = 'Teleport-Mfa-Response';

export class ApiService {
  auth: AuthService;

  constructor(ctx: TeleportContext) {
    this.auth = ctx.authService;
  }

  static defaultRequestOptions: RequestInit = {
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json; charset=utf-8',
    },
    mode: 'same-origin',
    cache: 'no-store',
  };

  getAuthHeaders(): Record<string, string> {
    const accessToken = ApiService.getAccessToken();
    const csrfToken = ApiService.getXCSRFToken();
    return {
      'X-CSRF-Token': csrfToken,
      Authorization: `Bearer ${accessToken}`,
    };
  }

  static getNoCacheHeaders(): Record<string, string> {
    return {
      'cache-control': 'max-age=0',
      expires: '0',
      pragma: 'no-cache',
    };
  }

  static getXCSRFToken(): string {
    const metaTag = document.querySelector(
      '[name=grv_csrf_token]'
    ) as HTMLMetaElement;
    return metaTag ? metaTag.content : '';
  }

  static getAccessToken(): string {
    return storageService.getBearerToken()?.accessToken;
  }

  static getHostName(): string {
    return location.hostname + (location.port ? ':' + location.port : '');
  }

  static isAdminActionRequiresMfaError(errMessage: string): boolean {
    return errMessage.includes(
      'admin-level API request requires MFA verification'
    );
  }

  static isRoleNotFoundError(errMessage: string): boolean {
    return /role \S+ is not found/.test(errMessage);
  }

  async get(url: string, abortSignal?: AbortSignal) {
    return this.fetchJsonWithMfaAuthnRetry(url, { signal: abortSignal });
  }

  async post(
    url: string,
    data?: unknown,
    abortSignal?: AbortSignal,
    mfaResponse?: MfaChallengeResponse
  ) {
    return this.fetchJsonWithMfaAuthnRetry(
      url,
      {
        body: JSON.stringify(data),
        method: 'POST',
        signal: abortSignal,
      },
      mfaResponse
    );
  }

  async postFormData(
    url: string,
    formData: FormData,
    mfaResponse?: MfaChallengeResponse
  ) {
    if (!(formData instanceof FormData)) {
      throw new Error('Data for body is not a type of FormData');
    }

    return this.fetchJsonWithMfaAuthnRetry(
      url,
      {
        body: formData,
        method: 'POST',
      },
      mfaResponse
    );
  }

  async delete(
    url: string,
    data?: unknown,
    mfaResponse?: MfaChallengeResponse
  ) {
    return this.fetchJsonWithMfaAuthnRetry(
      url,
      {
        body: JSON.stringify(data),
        method: 'DELETE',
      },
      mfaResponse
    );
  }

  async deleteWithHeaders(
    url: string,
    headers?: Record<string, string>,
    signal?: AbortSignal,
    mfaResponse?: MfaChallengeResponse
  ) {
    return this.fetchJsonWithMfaAuthnRetry(
      url,
      {
        method: 'DELETE',
        headers,
        signal,
      },
      mfaResponse
    );
  }

  async put(url: string, data: unknown, mfaResponse?: MfaChallengeResponse) {
    return this.fetchJsonWithMfaAuthnRetry(
      url,
      {
        body: JSON.stringify(data),
        method: 'PUT',
      },
      mfaResponse
    );
  }

  async putWithHeaders(
    url: string,
    data: unknown,
    headers?: Record<string, string>,
    mfaResponse?: MfaChallengeResponse
  ) {
    return this.fetchJsonWithMfaAuthnRetry(
      url,
      {
        body: JSON.stringify(data),
        method: 'PUT',
        headers,
      },
      mfaResponse
    );
  }

  async fetchJsonWithMfaAuthnRetry(
    url: string,
    customOptions: RequestInit,
    mfaResponse?: MfaChallengeResponse
  ): Promise<any> {
    const response = await this.fetch(url, customOptions, mfaResponse);

    let json;
    try {
      json = await response.json();
    } catch (err) {
      const message = response.ok
        ? err.message
        : `${response.status} - ${response.url}`;
      throw new ApiError({ message, response, opts: { cause: err } });
    }

    if (response.ok) {
      return json;
    }

    if (ApiService.isRoleNotFoundError(parseError(json))) {
      websession.logoutWithoutSlo({
        rememberLocation: false,
        withAccessChangedMessage: true,
      });
      return;
    }

    const isAdminActionMfaError = ApiService.isAdminActionRequiresMfaError(
      parseError(json)
    );
    if (!isAdminActionMfaError || mfaResponse) {
      throw new ApiError({
        message: parseError(json),
        response,
        proxyVersion: parseProxyVersion(json),
        messages: json.messages,
      });
    }

    const challenge = await this.auth.getMfaChallenge({
      scope: MfaChallengeScope.ADMIN_ACTION,
    });
    const mfaResponseForRetry =
      await this.auth.getMfaChallengeResponse(challenge);

    return this.fetchJsonWithMfaAuthnRetry(
      url,
      customOptions,
      mfaResponseForRetry
    );
  }

  fetch(
    url: string,
    customOptions: RequestInit = {},
    mfaResponse?: MfaChallengeResponse
  ): Promise<Response> {
    url = window.location.origin + url;

    const options: RequestInit = {
      ...ApiService.defaultRequestOptions,
      ...customOptions,
      headers: {
        ...ApiService.defaultRequestOptions.headers,
        ...this.getAuthHeaders(),
        ...customOptions.headers,
        ...(mfaResponse && {
          [MFA_HEADER]: JSON.stringify({
            ...mfaResponse,
            webauthnAssertionResponse: mfaResponse.webauthn_response,
          }),
        }),
      },
    };

    return fetch(url, options);
  }
}
