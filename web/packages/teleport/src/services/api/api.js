/*
Copyright 2015 Gravitational, Inc.

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
import 'whatwg-fetch';
import { storageService } from '../storageService';

import parseError, { ApiError } from './parseError';
import auth from 'teleport/services/auth/auth';

const api = {
  get(url, abortSignal) {
    return api.fetchJson(url, { signal: abortSignal });
  },

  post(url, data, abortSignal) {
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

  delete(url, data) {
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

  async fetchJson(url, params, withMFA) {
    if (withMFA) {
      // Get an MFA response and add it to the request headers.
      const webauthn = auth.getWebauthnResponse();
      params.headers = {
        ...params.headers,
        'Mfa-Response': JSON.stringify({
          webauthnAssertionResponse: webauthn,
        }),
      };
    }

    return new Promise((resolve, reject) => {
      this.fetch(url, params)
        .then(response => {
          if (response.ok) {
            return response
              .json()
              .then(json => resolve(json))
              .catch(err =>
                reject(new ApiError(err.message, response, { cause: err }))
              );
          } else {
            return response
              .json()
              .then(json => {
                if (
                  !withMFA &&
                  isAdminActionRequiresMFAError(parseError(json))
                ) {
                  // Retry with MFA.
                  return this.fetchJson(url, params, true)
                    .then(resp => resolve(resp))
                    .catch(err => reject(err));
                }
                reject(new ApiError(parseError(json), response));
              })
              .catch(err =>
                reject(
                  new ApiError(
                    `${response.status} - ${response.url}`,
                    response,
                    { cause: err }
                  )
                )
              );
          }
        })
        .catch(err => {
          reject(err);
        });
    });
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

const requestOptions = {
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
  const metaTag = document.querySelector('[name=grv_csrf_token]');
  return metaTag ? metaTag.content : '';
};

export function getAccessToken() {
  const bearerToken = storageService.getBearerToken() || {};
  return bearerToken.accessToken;
}

export function getHostName() {
  return location.hostname + (location.port ? ':' + location.port : '');
}

function isAdminActionRequiresMFAError(errMessage) {
  return errMessage.includes(
    'admin-level API request requires MFA verification'
  );
}

export default api;
