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

import { z, ZodIssue } from 'zod';
import zodToJsonSchema from 'zod-to-json-schema';

import Logger from 'teleterm/logger';
import { RuntimeSettings } from 'teleterm/mainProcess/types';
import { FileStorage } from 'teleterm/services/fileStorage';

import {
  AppConfig,
  AppConfigSchema,
  createAppConfigSchema,
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

// createConfigService must return a client that works both in the browser and in Node.js, as the
// returned service is used both in the main process and in Storybook to provide a fake
// implementation of config service.
export function createConfigService({
  configFile,
  jsonSchemaFile,
  settings,
}: {
  configFile: FileStorage;
  jsonSchemaFile: FileStorage;
  settings: RuntimeSettings;
}): ConfigService {
  const schema = createAppConfigSchema(settings);
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
  const jsonSchemaFileName = jsonSchemaFile.getFileName();
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
