import fs from 'fs';
import crypto from 'crypto';

const UUID_V4_REGEX =
  /^[0-9A-F]{8}-[0-9A-F]{4}-4[0-9A-F]{3}-[89AB][0-9A-F]{3}-[0-9A-F]{12}$/i;

/**
 * Returns a unique ID (UUIDv4) of the installed app. The ID is stored in a file
 * under a specified path. If the file containing the value does not exist or has
 * an invalid format then it is automatically (re-)generated.
 */
export function loadInstallationId(filePath: string): string {
  let id = '';
  try {
    id = fs.readFileSync(filePath, 'utf-8');
  } catch (error) {
    return writeInstallationId(filePath);
  }
  if (!UUID_V4_REGEX.test(id)) {
    return writeInstallationId(filePath);
  }
  return id;
}

function writeInstallationId(filePath: string): string {
  const newId = crypto.randomUUID();
  try {
    fs.writeFileSync(filePath, newId);
  } catch (error) {
    throw new Error(
      `Could not write installation_id to ${filePath}, ${error.message}`
    );
  }
  return newId;
}
