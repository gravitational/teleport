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
import auth from 'teleport/services/auth/auth';

import { storageService } from '../storageService';

import parseError, { ApiError } from './parseError';

const MFA_HEADER = 'Teleport-Mfa-Response';

const api = {
  get(url, abortSignal?) {
    return api.fetchJson(url, { signal: abortSignal });
  },

  post(url, data?, abortSignal?) {
    return api.fetchJson(url, {
      body: JSON.stringify(data),
      method: 'POST',
      signal: abortSignal,
    });
  },

  postFormData(url, formData) {
    if (formData instanceof FormData) {
      return api.fetchJson(url, {
        body: formData,
        method: 'POST',
        // Overrides the default header from `requestOptions`.
        headers: {
          Accept: 'application/json',
          // Let the browser infer the content-type for FormData types
          // to set the correct boundary:
          // 1) https://developer.mozilla.org/en-US/docs/Web/API/FormData/Using_FormData_Objects#sending_files_using_a_formdata_object
          // 2) https://stackoverflow.com/a/64653976
        },
      });
    }

    throw new Error('data for body is not a type of FormData');
  },

  delete(url, data?) {
    return api.fetchJson(url, {
      body: JSON.stringify(data),
      method: 'DELETE',
    });
  },

  put(url, data) {
    return api.fetchJson(url, {
      body: JSON.stringify(data),
      method: 'PUT',
    });
  },

  async fetchJson(url, params): Promise<any> {
    const response = await api.fetch(url, params);

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
    const isMfaHeaderPresent = params.headers?.[MFA_HEADER];
    const shouldRetry = isAdminActionMfaError && !isMfaHeaderPresent;
    if (!shouldRetry) {
      throw new ApiError(parseError(json), response);
    }

    let webauthnResponse;
    try {
      webauthnResponse = await auth.getWebauthnResponse();
    } catch (err) {
      throw new Error(
        'Failed to fetch webauthn credentials, please connect a registered hardware key and try again. If you do not have a hardware key registered, you can add one from your account settings page.'
      );
    }

    const paramsWithMfaHeader = {
      ...params,
      headers: {
        ...params.headers,
        [MFA_HEADER]: JSON.stringify({
          webauthnAssertionResponse: webauthnResponse,
        }),
      },
    };
    return api.fetchJson(url, paramsWithMfaHeader);
  },

  fetch(url, params = {}) {
    url = window.location.origin + url;
    const options = {
      ...requestOptions,
      ...params,
    };

    options.headers = {
      ...requestOptions.headers,
      ...options.headers,
      ...getAuthHeaders(),
    };

    // native call
    return fetch(url, options);
  },
};

const requestOptions: RequestInit = {
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
