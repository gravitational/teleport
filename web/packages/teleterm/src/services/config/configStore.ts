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

import { ZodIssue } from 'zod';

import { FileStorage } from 'teleterm/services/fileStorage';
import Logger from 'teleterm/logger';
import { Platform } from 'teleterm/mainProcess/types';

import { createAppConfigSchema, AppConfig } from './createAppConfigSchema';

const logger = new Logger('ConfigService');

export interface ConfigService {
  get<K extends keyof AppConfig>(
    key: K
  ): { value: AppConfig[K]; metadata: { isStored: boolean } };

  set<K extends keyof AppConfig>(key: K, value: AppConfig[K]): void;

  getStoredConfigErrors(): ZodIssue[] | undefined;
}

export function createConfigService({
  configFile,
  jsonSchemaFile,
  platform,
}: {
  configFile: FileStorage;
  jsonSchemaFile: FileStorage;
  platform: Platform;
}): ConfigService {
  const schema = createAppConfigSchema(platform);
  const { storedConfig, configWithDefaults, errors } = validateStoredConfig();

  function parse(data: Partial<AppConfig>) {
    return schema.safeParse(data);
  }

  //TODO (gzdunek): syntax errors of the JSON file are silently ignored, report
  // them to the user too
  function validateStoredConfig(): {
    storedConfig: Partial<AppConfig>;
    configWithDefaults: AppConfig;
    errors: ZodIssue[] | undefined;
  } {
    const storedConfig = configFile.get<Partial<AppConfig>>();
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
      // it can happen when a default value does not pass validation
      throw new Error(
        `Re-parsing config file failed \n${reParsed.error.message}`
      );
    }
    return {
      storedConfig: withoutInvalidKeys,
      configWithDefaults: reParsed.data,
      errors: parsed.error.issues,
    };
  }

  return {
    get: key => ({
      value: configWithDefaults[key],
      metadata: { isStored: storedConfig[key] !== undefined },
    }),
    set: (key, value) => {
      configFile.put(key as string, value);
      configWithDefaults[key] = value;
      storedConfig[key] = value;
    },
    getStoredConfigErrors: () => errors,
  };
}
