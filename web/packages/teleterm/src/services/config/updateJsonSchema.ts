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

import { z } from 'zod';

import zodToJsonSchema from 'zod-to-json-schema';

import { FileStorage } from 'teleterm/services/fileStorage';

export function updateJsonSchema({
  configSchema,
  configFile,
  jsonSchemaFile,
}: {
  configSchema: z.AnyZodObject;
  configFile: FileStorage;
  jsonSchemaFile: FileStorage;
}): void {
  //adds $schema field to the original schema to prevent marking it as a not allowed property
  const configSchemaWithSchemaField = configSchema.extend({
    $schema: z.string(),
  });
  const jsonSchema = zodToJsonSchema(configSchemaWithSchemaField);

  jsonSchemaFile.replace(jsonSchema);
  linkToJsonSchemaIfNeeded(
    configFile,
    path.basename(jsonSchemaFile.getFilePath())
  );
}

function linkToJsonSchemaIfNeeded(
  configFileStorage: FileStorage,
  jsonSchemaFilename: string
): void {
  const schemaField = configFileStorage.get('$schema');
  if (schemaField !== jsonSchemaFilename) {
    configFileStorage.put('$schema', jsonSchemaFilename);
  }
}
