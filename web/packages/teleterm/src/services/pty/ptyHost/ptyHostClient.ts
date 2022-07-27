import { ChannelCredentials, Metadata } from '@grpc/grpc-js';
import { Struct } from 'google-protobuf/google/protobuf/struct_pb';
import {
  PtyHostClient as GrpcClient,
  PtyCreate,
  PtyId,
} from 'teleterm/sharedProcess/ptyHost';
import { PtyEventsStreamHandler } from './ptyEventsStreamHandler';
import { PtyHostClient } from '../types';

export function createPtyHostClient(
  address: string,
  credentials: ChannelCredentials
): PtyHostClient {
  const client = new GrpcClient(address, credentials);
  return {
    createPtyProcess(ptyOptions) {
      const request = new PtyCreate()
        .setArgsList(ptyOptions.args)
        .setPath(ptyOptions.path)
        .setEnv(Struct.fromJavaScript(ptyOptions.env));

      if (ptyOptions.cwd) {
        request.setCwd(ptyOptions.cwd);
      }

      if (ptyOptions.initCommand) {
        request.setInitCommand(ptyOptions.initCommand);
      }

      return new Promise<string>((resolve, reject) => {
        client.createPtyProcess(request, (error, response) => {
          if (error) {
            reject(error);
          } else {
            resolve(response.toObject().id);
          }
        });
      });
    },
    getCwd(ptyId) {
      return new Promise((resolve, reject) => {
        client.getCwd(new PtyId().setId(ptyId), (error, response) => {
          if (error) {
            reject(error);
          } else {
            resolve(response.toObject().cwd);
          }
        });
      });
    },
    exchangeEvents(ptyId) {
      const metadata = new Metadata();
      metadata.set('ptyId', ptyId);
      const stream = client.exchangeEvents(metadata);
      return new PtyEventsStreamHandler(stream);
    },
  };
}
