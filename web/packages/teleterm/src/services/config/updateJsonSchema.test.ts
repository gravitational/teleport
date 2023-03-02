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
  const configJsonSchemaFile = createMockFileStorage({
    filePath: '~/config_schema.json',
  });

  updateJsonSchema({
    configSchema: schema,
    configFile: configFile,
    configJsonSchemaFile: configJsonSchemaFile,
  });

  expect(configFile.get('$schema')).toBe('config_schema.json');
  expect(configJsonSchemaFile.get()).toEqual(generatedJsonSchema);
});
