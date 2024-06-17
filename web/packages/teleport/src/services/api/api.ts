/*
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

import 'whatwg-fetch';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';

import { storageService } from '../storageService';
import { WebauthnAssertionResponse } from '../auth';

import parseError, { ApiError } from './parseError';

export const MFA_HEADER = 'Teleport-Mfa-Response';

const api = {
  get(url, abortSignal?) {
    return api.fetchJsonWithMfaAuthnRetry(url, { signal: abortSignal });
  },

  post(url, data?, abortSignal?, webauthnResponse?: WebauthnAssertionResponse) {
    return api.fetchJsonWithMfaAuthnRetry(
      url,
      {
        body: JSON.stringify(data),
        method: 'POST',
        signal: abortSignal,
      },
      webauthnResponse
    );
  },

  postFormData(url, formData, webauthnResponse?: WebauthnAssertionResponse) {
    if (formData instanceof FormData) {
      return api.fetchJsonWithMfaAuthnRetry(
        url,
        {
          body: formData,
          method: 'POST',
          // Overrides the default header from `defaultRequestOptions`.
          headers: {
            Accept: 'application/json',
            // Let the browser infer the content-type for FormData types
            // to set the correct boundary:
            // 1) https://developer.mozilla.org/en-US/docs/Web/API/FormData/Using_FormData_Objects#sending_files_using_a_formdata_object
            // 2) https://stackoverflow.com/a/64653976
          },
        },
        webauthnResponse
      );
    }

    throw new Error('data for body is not a type of FormData');
  },

  delete(url, data?, webauthnResponse?: WebauthnAssertionResponse) {
    return api.fetchJsonWithMfaAuthnRetry(
      url,
      {
        body: JSON.stringify(data),
        method: 'DELETE',
      },
      webauthnResponse
    );
  },

  deleteWithHeaders(url, headers?: Record<string, string>, signal?) {
    return api.fetch(url, {
      method: 'DELETE',
      headers,
      signal,
    });
  },

  put(url, data, webauthnResponse?: WebauthnAssertionResponse) {
    return api.fetchJsonWithMfaAuthnRetry(
      url,
      {
        body: JSON.stringify(data),
        method: 'PUT',
      },
      webauthnResponse
    );
  },

  /**
   * fetchJsonWithMfaAuthnRetry calls on `api.fetch` and
   * processes the response.
   *
   * It returns the JSON data if it is a valid JSON and
   * there were no response errors.
   *
   * If a response had an error and it contained a MFA authn
   * required message, then a retry is attempted after a user
   * successfully re-authenticates with an MFA device.
   *
   * All other errors will be thrown.
   */
  async fetchJsonWithMfaAuthnRetry(
    url: string,
    customOptions: RequestInit,
    webauthnResponse?: WebauthnAssertionResponse
  ): Promise<any> {
    const response = await api.fetch(url, customOptions, webauthnResponse);

    let json;
    try {
      json = await response.json();
    } catch (err) {
      const message = response.ok
        ? err.message
        : `${response.status} - ${response.url}`;
      throw new ApiError(message, response, { cause: err });
    }

    if (response.ok) {
      return json;
    }

    // Retry with MFA if we get an admin action missing MFA error.
    const isAdminActionMfaError = isAdminActionRequiresMfaError(
      parseError(json)
    );
    const shouldRetry = isAdminActionMfaError && !webauthnResponse;
    if (!shouldRetry) {
      throw new ApiError(parseError(json), response, undefined, json.messages);
    }

    let webauthnResponseForRetry;
    try {
      webauthnResponseForRetry = await auth.getWebauthnResponse(
        MfaChallengeScope.ADMIN_ACTION
      );
    } catch (err) {
      throw new Error(
        'Failed to fetch webauthn credentials, please connect a registered hardware key and try again. If you do not have a hardware key registered, you can add one from your account settings page.'
      );
    }

    return api.fetchJsonWithMfaAuthnRetry(
      url,
      customOptions,
      webauthnResponseForRetry
    );
  },

  /**
   * fetch calls upon native fetch with options and headers set
   * to default (or provided) values.
   *
   * @param customOptions is an optional RequestInit object.
   * It can be provided to either override some fields defined in
   * `defaultRequestOptions` or define new fields not in
   * `defaultRequestOptions`.
   *
   * customOptions gets shallowly merged with `defaultRequestOptions` where
   * inner objects do not get merged if overrided.
   *
   * Example with an example customOption:
   * {
   *  body: formData,
   *  method: 'POST',
   *  headers: {
   *    Accept: 'application/json',
   *  }
   * }
   *
   * 'headers' is a field also defined in `defaultRequestOptions`, because of
   * shallow merging, the customOption.headers will get completely overrided.
   * After merge:
   *
   * {
   *  body: formData,
   *  method: 'POST',
   *  headers: {
   *    Accept: 'application/json',
   *  },
   *  credentials: 'same-origin',
   *  mode: 'same-origin',
   *  cache: 'no-store'
   * }
   *
   * If customOptions field is not provided, only fields defined in
   * `defaultRequestOptions` will be used.
   *
   * @param webauthnResponse if defined (eg: `fetchJsonWithMfaAuthnRetry`)
   * will add a custom MFA header field that will hold the webauthn response.
   */
  fetch(
    url: string,
    customOptions: RequestInit = {},
    webauthnResponse?: WebauthnAssertionResponse
  ) {
    url = window.location.origin + url;
    const options = {
      ...defaultRequestOptions,
      ...customOptions,
    };

    options.headers = {
      ...options.headers,
      ...getAuthHeaders(),
    };

    if (webauthnResponse) {
      options.headers[MFA_HEADER] = JSON.stringify({
        webauthnAssertionResponse: webauthnResponse,
      });
    }

    // native call
    return fetch(url, options);
  },
};

export const defaultRequestOptions: RequestInit = {
  credentials: 'same-origin',
  headers: {
    Accept: 'application/json',
    'Content-Type': 'application/json; charset=utf-8',
  },
  mode: 'same-origin',
  cache: 'no-store',
};

export function getAuthHeaders() {
  const accessToken = getAccessToken();
  const csrfToken = getXCSRFToken();
  return {
    'X-CSRF-Token': csrfToken,
    Authorization: `Bearer ${accessToken}`,
  };
}

export function getNoCacheHeaders() {
  return {
    'cache-control': 'max-age=0',
    expires: '0',
    pragma: 'no-cache',
  };
}

export const getXCSRFToken = () => {
  const metaTag = document.querySelector(
    '[name=grv_csrf_token]'
  ) as HTMLMetaElement;
  return metaTag ? metaTag.content : '';
};

export function getAccessToken() {
  return storageService.getBearerToken()?.accessToken;
}

export function getHostName() {
  return location.hostname + (location.port ? ':' + location.port : '');
}

function isAdminActionRequiresMfaError(errMessage) {
  return errMessage.includes(
    'admin-level API request requires MFA verification'
  );
}

export default api;
