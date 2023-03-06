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
  schema,
  configFile,
  jsonSchemaFile,
}: {
  schema: z.AnyZodObject;
  configFile: FileStorage;
  jsonSchemaFile: FileStorage;
}): void {
  const jsonSchema = zodToJsonSchema(
    // Add $schema field to prevent marking it as a not allowed property.
    schema.extend({ $schema: z.string() })
  );
  const jsonSchemaFileName = path.basename(jsonSchemaFile.getFilePath());
  const jsonSchemaFileNameInConfig = configFile.get('$schema');

  jsonSchemaFile.replace(jsonSchema);

  if (jsonSchemaFileNameInConfig !== jsonSchemaFileName) {
    configFile.put('$schema', jsonSchemaFileName);
  }
}
