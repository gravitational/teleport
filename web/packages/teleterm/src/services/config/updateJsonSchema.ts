import path from 'path';

import { z } from 'zod';

import zodToJsonSchema from 'zod-to-json-schema';

import { FileStorage } from 'teleterm/services/fileStorage';

export function updateJsonSchema({
  configSchema,
  configFile,
  configJsonSchemaFile,
}: {
  configSchema: z.AnyZodObject;
  configFile: FileStorage;
  configJsonSchemaFile: FileStorage;
}): void {
  //adds $schema field to the original schema to prevent marking it as a not allowed property
  const configSchemaWithSchemaField = configSchema.extend({
    $schema: z.string(),
  });
  const jsonSchema = zodToJsonSchema(configSchemaWithSchemaField);

  configJsonSchemaFile.replace(jsonSchema);
  linkToJsonSchemaIfNeeded(
    configFile,
    path.basename(configJsonSchemaFile.getFilePath())
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
