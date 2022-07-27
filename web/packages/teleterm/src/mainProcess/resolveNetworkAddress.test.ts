import { fork, spawn } from 'child_process';
import path from 'path';

import { resolveNetworkAddress } from './resolveNetworkAddress';

// Hardcoded in testProcess.js.
const PORT = '1337';

it('returns an error when supplied an unknown protocol', async () => {
  const process = fork(path.join(__dirname, 'testProcess.js'), {
    silent: true,
  });

  try {
    const result = resolveNetworkAddress(
      'unknown-protocol://localhost:1237',
      process
    );

    await expect(result).rejects.toThrow('Unknown protocol unknown-protocol');
  } finally {
    process.kill();
  }
});

const testSuites = [
  {
    name: 'UDS',
    requestedAddress: 'unix:///tmp/test',
    expectedNetworkAddress: 'unix:///tmp/test',
  },
  {
    name: 'TCP',
    requestedAddress: 'tcp://localhost:0',
    expectedNetworkAddress: `localhost:${PORT}`,
  },
];

describe.each(testSuites)(
  'for $name process',
  ({ requestedAddress, expectedNetworkAddress }) => {
    it(`waits for the process to output the matching string`, async () => {
      const process = fork(path.join(__dirname, 'testProcess.js'), {
        silent: true,
      });

      try {
        const actualNetworkAddress = await resolveNetworkAddress(
          requestedAddress,
          process
        );

        expect(actualNetworkAddress).toEqual(expectedNetworkAddress);
      } finally {
        process.kill();
      }
    });

    it(`times out if the process doesn't return the match in time`, async () => {
      const process = fork(path.join(__dirname, 'testProcess.js'), ['100'], {
        silent: true,
      });

      try {
        await expect(
          resolveNetworkAddress(requestedAddress, process, 10)
        ).rejects.toThrow('operation timed out');
      } finally {
        process.kill();
      }
    });

    it(`returns an error if the process exits without returning a match`, async () => {
      const process = fork(
        path.join(__dirname, 'testProcess.js'),
        ['10', 'exit-prematurely'],
        {
          silent: true,
        }
      );

      try {
        await expect(
          resolveNetworkAddress(requestedAddress, process)
        ).rejects.toThrow('the process exited with code 1');
      } finally {
        process.kill();
      }
    });

    it(`returns an error if the process fails to start`, async () => {
      const process = spawn(path.join(__dirname, 'testProcess-nonExistent.js'));

      try {
        await expect(
          resolveNetworkAddress('tcp://localhost:0', process)
        ).rejects.toThrow('testProcess-nonExistent.js ENOENT');
      } finally {
        process.kill();
      }
    });
  }
);
