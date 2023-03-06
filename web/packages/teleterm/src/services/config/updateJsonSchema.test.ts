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

import { z } from 'zod';

import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';

import { updateJsonSchema } from './updateJsonSchema';

const schema = z.object({
  'fonts.monoFamily': z.string().default('Arial'),
  'usageReporting.enabled': z.boolean().default(false),
});

const generatedJsonSchema = {
  $schema: 'http://json-schema.org/draft-07/schema#',
  additionalProperties: false,
  properties: {
    $schema: {
      type: 'string',
    },
    'fonts.monoFamily': {
      default: 'Arial',
      type: 'string',
    },
    'usageReporting.enabled': {
      default: false,
      type: 'boolean',
    },
  },
  required: ['$schema'],
  type: 'object',
};

test('field linking to the schema and the schema itself are updated', () => {
  const configFile = createMockFileStorage();
  const jsonSchemaFile = createMockFileStorage({
    filePath: '~/config_schema.json',
  });

  updateJsonSchema({
    schema,
    configFile,
    jsonSchemaFile,
  });

  expect(configFile.get('$schema')).toBe('config_schema.json');
  expect(jsonSchemaFile.get()).toEqual(generatedJsonSchema);
});
