import { Server, ServerCredentials } from '@grpc/grpc-js';
import { PtyHostService } from './ptyHost';
import { createPtyHostService } from './ptyHost/ptyHostService';

function createServer(): void {
  const args = process.argv.slice(2);
  const address =
    args[0].startsWith('--addr=') && args[0].replace('--addr=', '');

  if (!address) {
    throw new Error('Provide gRPC server address');
  }

  process.on('uncaughtException', console.error);

  const server = new Server();
  // @ts-expect-error we have a typed service
  server.addService(PtyHostService, createPtyHostService());
  server.bindAsync(address, ServerCredentials.createInsecure(), error => {
    console.error(error);
    server.start();
  });

  process.once('exit', () => {
    server.forceShutdown();
  });
}

createServer();
