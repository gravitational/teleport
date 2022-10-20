import path from 'path';
import { watch } from 'fs';
import { readFile, writeFile, stat, rename } from 'fs/promises';

import { makeCert } from './makeCert';

/**
 * Generates self-signed cert and saves it in the `certsDir`
 * using `certName` (only the cert is saved).
 * The generated pair (cert and key) is returned.
 */
export async function generateAndSaveGrpcCert(
  certsDir: string,
  certName: string
): Promise<{ cert: Buffer; key: Buffer }> {
  const createdCert = await makeCert({
    commonName: 'localhost',
    validityDays: 365,
  });

  // File is first saved using under `tempFullPath` and then renamed to `fullPath`.
  // It prevents from reading half written file.
  const fullPath = path.join(certsDir, certName);
  const tempFullPath = fullPath + '.tmp';
  await writeFile(tempFullPath, createdCert.cert);
  await rename(tempFullPath, fullPath);

  return {
    cert: Buffer.from(createdCert.cert),
    key: Buffer.from(createdCert.key),
  };
}

/**
 * Reads a cert with given `certName` in the `certDir`.
 * If the file doesn't exist, it will wait up to 10 seconds for it.
 */
export async function readGrpcCert(
  certsDir: string,
  certName: string
): Promise<Buffer> {
  const fullPath = path.join(certsDir, certName);
  const abortController = new AbortController();

  async function fileExistsAndHasSize(): Promise<boolean> {
    return !!(await stat(fullPath)).size;
  }

  function watchForFile(): Promise<Buffer> {
    return new Promise((resolve, reject) => {
      abortController.signal.onabort = () => {
        watcher.close();
        clearTimeout(timeout);
      };

      const timeout = setTimeout(() => {
        reject(
          `Could not read ${certName} certificate. The operation timed out.`
        );
      }, 10_000);

      const watcher = watch(certsDir, async (event, filename) => {
        if (certName === filename && (await fileExistsAndHasSize())) {
          resolve(readFile(fullPath));
        }
      });
    });
  }

  async function checkIfFileAlreadyExists(): Promise<Buffer> {
    if (await fileExistsAndHasSize()) {
      return readFile(fullPath);
    }
  }

  try {
    // watching must be started before checking if the file already exists to avoid race conditions
    return await Promise.any([watchForFile(), checkIfFileAlreadyExists()]);
  } finally {
    abortController.abort();
  }
}
