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

import { RpcInterceptor } from '@protobuf-ts/runtime-rpc';

import { isObject } from 'shared/utils/highbar';

import Logger from 'teleterm/logger';

const SENSITIVE_PROPERTIES = ['passw', 'authClusterId'];

export function loggingInterceptor(logger: Logger): RpcInterceptor {
  return {
    interceptUnary: (next, method, input, options) => {
      const output = next(method, input, options);
      const { logRequest, logResponse, logError } = makeMethodLogger(
        logger,
        method.name
      );

      logRequest(input);
      output
        .then(({ response }) => logResponse(response))
        .catch(error => logError(error));

      return output;
    },
    interceptClientStreaming: (next, method, options) => {
      const output = next(method, options);
      const { logRequest, logResponse, logError } = makeMethodLogger(
        logger,
        method.name
      );

      const originalSend = output.requests.send.bind(output.requests);
      output.requests.send = message => {
        logRequest(message);
        return originalSend(message);
      };

      output
        .then(({ response }) => logResponse(response))
        .catch(error => logError(error));
      return output;
    },
    interceptServerStreaming: (next, method, input, options) => {
      const output = next(method, input, options);
      const { logRequest, logResponse, logError } = makeMethodLogger(
        logger,
        method.name
      );

      logRequest(input);
      output.responses.onNext((message, error) => {
        if (message) {
          logResponse(message);
        }
        if (error) {
          logError(error);
        }
      });

      return output;
    },
    interceptDuplex: (next, method, options) => {
      const output = next(method, options);
      const { logRequest, logResponse, logError } = makeMethodLogger(
        logger,
        method.name
      );

      const originalSend = output.requests.send.bind(output.requests);
      output.requests.send = message => {
        logRequest(message);
        return originalSend(message);
      };

      output.responses.onNext((message, error) => {
        if (message) {
          logResponse(message);
        }
        if (error) {
          logError(error);
        }
      });

      return output;
    },
  };
}

export function filterSensitiveProperties(toFilter: object): object {
  const acc = {};
  const transformer = (result: object, value: any, key: any) => {
    if (
      SENSITIVE_PROPERTIES.some(
        sensitiveProp => typeof key === 'string' && key.includes(sensitiveProp)
      )
    ) {
      result[key] = '~FILTERED~';
      return;
    }
    if (isObject(value)) {
      result[key] = filterSensitiveProperties(value);
      return;
    }
    result[key] = value;
  };

  Object.keys(toFilter).forEach(key => transformer(acc, toFilter[key], key));

  return acc;
}

function makeMethodLogger(logger: Logger, methodName: string) {
  return {
    logRequest: (input: object) => {
      logger.info(`${methodName} REQUEST:`, filterSensitiveProperties(input));
    },
    logResponse: (output: object) => {
      const toLog = output ? filterSensitiveProperties(output) : null;
      logger.info(`${methodName} RESPONSE:`, toLog);
    },
    logError: (error: unknown) => {
      logger.error(`${methodName} RESPONSE:`, `${error}`);
    },
  };
}
