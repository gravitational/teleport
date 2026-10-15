import { sleep } from 'k6';
import exec from 'k6/execution';
import http from 'k6/http';
import { Counter } from 'k6/metrics';

import { TokenExchange } from './lib/auth.js';
import { ScimClientConfig } from './lib/config.js';
import { Client } from './lib/scim.js';

/**
 * REQUEST_TAG is the k6 tag key carrying each request's RequestType.
 */
export const REQUEST_TAG = 'request_type';

/**
 * retries counts 429 responses retried after Retry-After, tagged by request type.
 */
const retries = new Counter('scim_retries');

/**
 * transport sends requests with k6/http, tagging each with its request type.
 * Unset headers or timeout fall back to k6's defaults.
 */
const transport = {
  request(method, url, body, { headers, timeout, requestType }) {
    const params = { tags: { [REQUEST_TAG]: requestType } };
    if (headers) {
      params.headers = headers;
    }
    if (timeout) {
      params.timeout = timeout;
    }
    return http.request(method, url, body, params);
  },
};

/**
 * loadScimClientConfig returns the connection settings shared by all scripts.
 */
export function loadScimClientConfig() {
  return ScimClientConfig.fromEnv(env);
}

/**
 * newClient creates a new SCIM client.
 */
export function newClient(cfg) {
  const tokens = new TokenExchange(cfg, transport);
  return new Client(cfg, {
    transport,
    tokens: {
      token() {
        try {
          return tokens.token();
        } catch (err) {
          exec.test.abort(err.message);
        }
      },
    },
    sleep,
    onRetry: requestType => retries.add(1, { [REQUEST_TAG]: requestType }),
  });
}

/**
 * k6 request/response timing metric.
 */
export function timings(res) {
  const { duration, waiting, receiving } = res.timings;
  return (
    `${Math.round(duration)} ms ` +
    `(waiting ${Math.round(waiting)} ms, receiving ${Math.round(receiving)} ms)`
  );
}

class Env {
  constructor(vars) {
    this.vars = vars;
  }

  // required returns the named variable or throws when it is unset or empty.
  required(name) {
    const value = this.vars[name];
    if (!value) {
      throw new Error(`${name} must be set`);
    }
    return value;
  }

  // optional returns the named variable or the fallback.
  optional(name, fallback) {
    return this.vars[name] || fallback;
  }
}

/**
 * env reads the k6 environment (-e flags and exported variables).
 */
export const env = new Env(__ENV);
