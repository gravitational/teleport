/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { z, ZodIssue } from 'zod';

import { FileStorage } from 'teleterm/services/fileStorage';
import Logger from 'teleterm/logger';

const logger = new Logger('ConfigStore');

export function createConfigStore<
  Schema extends z.ZodTypeAny,
  Shape = z.infer<Schema>
>(schema: Schema, fileStorage: FileStorage) {
  const { storedConfig, configWithDefaults, errors } = validateStoredConfig();

  function get<K extends keyof Shape>(
    key: K
  ): { value: Shape[K]; metadata: { isStored: boolean } } {
    return {
      value: configWithDefaults[key],
      metadata: { isStored: storedConfig[key] !== undefined },
    };
  }

  function set<K extends keyof Shape>(key: K, value: Shape[K]): void {
    fileStorage.put(key as string, value);
    configWithDefaults[key] = value;
    storedConfig[key] = value;
  }

  function getStoredConfigErrors(): ZodIssue[] | undefined {
    return errors;
  }

  function parse(data: Partial<Shape>) {
    return schema.safeParse(data);
  }

  //TODO (gzdunek): syntax errors of the JSON file are silently ignored, report
  // them to the user too
  function validateStoredConfig(): {
    storedConfig: Partial<Shape>;
    configWithDefaults: Shape;
    errors: ZodIssue[] | undefined;
  } {
    const storedConfig = fileStorage.get<Partial<Shape>>();
    const parsed = parse(storedConfig);
    if (parsed.success === true) {
      return {
        storedConfig,
        configWithDefaults: parsed.data,
        errors: undefined,
      };
    }
    const withoutInvalidKeys = { ...storedConfig };
    parsed.error.issues.forEach(error => {
      // remove only top-level keys
      delete withoutInvalidKeys[error.path[0]];
      logger.info(
        `Invalid config key, error: ${error.message} at ${error.path.join('.')}`
      );
    });
    const reParsed = parse(withoutInvalidKeys);
    if (reParsed.success === false) {
      // it should not occur after removing invalid keys, but just in case
      throw new Error('Re-parsing config file failed', reParsed.error.cause);
    }
    return {
      storedConfig: withoutInvalidKeys,
      configWithDefaults: reParsed.data,
      errors: parsed.error.issues,
    };
  }

  return { get, set, getStoredConfigErrors };
}
