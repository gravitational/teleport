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

import path from 'path';

import { z, ZodIssue } from 'zod';
import zodToJsonSchema from 'zod-to-json-schema';

import { FileStorage } from 'teleterm/services/fileStorage';
import Logger from 'teleterm/logger';
import { Platform } from 'teleterm/mainProcess/types';

import {
  createAppConfigSchema,
  AppConfigSchema,
  AppConfig,
} from './appConfigSchema';

const logger = new Logger('ConfigService');

type FileLoadingError = {
  source: 'file-loading';
  error: Error;
};

type ValidationError = {
  source: 'validation';
  errors: ZodIssue[];
};

type ConfigError = FileLoadingError | ValidationError;

export interface ConfigService {
  get<K extends keyof AppConfig>(
    key: K
  ): { value: AppConfig[K]; metadata: { isStored: boolean } };

  set<K extends keyof AppConfig>(key: K, value: AppConfig[K]): void;

  /**
   * Returns validation errors or an error that occurred during loading the config file (this means IO and syntax errors).
   * This error has to be checked during the initialization of the app.
   *
   * The reason we have a getter for this error instead of making `createConfigService` fail with an error
   * is that in the presence of this error we want to notify about it and then continue with default values:
   * - If validation errors occur, the incorrect values are replaced with the defaults.
   * - In case of an error coming from loading the file, all values are replaced with the defaults.
   * */
  getConfigError(): ConfigError | undefined;
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
  updateJsonSchema({ schema, configFile, jsonSchemaFile });

  const {
    storedConfig,
    configWithDefaults,
    errors: validationErrors,
  } = validateStoredConfig(schema, configFile);

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
    getConfigError: () => {
      const fileLoadingError = configFile.getFileLoadingError();
      if (fileLoadingError) {
        return {
          source: 'file-loading',
          error: fileLoadingError,
        };
      }
      if (validationErrors) {
        return {
          source: 'validation',
          errors: validationErrors,
        };
      }
    },
  };
}

function updateJsonSchema({
  schema,
  configFile,
  jsonSchemaFile,
}: {
  schema: AppConfigSchema;
  configFile: FileStorage;
  jsonSchemaFile: FileStorage;
}): void {
  const jsonSchema = zodToJsonSchema(
    // Add $schema field to prevent marking it as a not allowed property.
    schema.extend({ $schema: z.string() }),
    { $refStrategy: 'none' }
  );
  const jsonSchemaFileName = path.basename(jsonSchemaFile.getFilePath());
  const jsonSchemaFileNameInConfig = configFile.get('$schema');

  jsonSchemaFile.replace(jsonSchema);

  if (jsonSchemaFileNameInConfig !== jsonSchemaFileName) {
    configFile.put('$schema', jsonSchemaFileName);
  }
}

function validateStoredConfig(
  schema: AppConfigSchema,
  configFile: FileStorage
): {
  storedConfig: Partial<AppConfig>;
  configWithDefaults: AppConfig;
  errors: ZodIssue[] | undefined;
} {
  const parse = (data: Partial<AppConfig>) => schema.safeParse(data);

  const storedConfig = configFile.get() as Partial<AppConfig>;
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
