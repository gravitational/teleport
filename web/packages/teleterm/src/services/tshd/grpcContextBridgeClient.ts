import {
  RpcInputStream,
  UnaryCall,
  ClientStreamingCall,
  ServerStreamingCall,
  DuplexStreamingCall,
  RpcInterceptor,
} from '@protobuf-ts/runtime-rpc';

import { ServiceInfo } from '@protobuf-ts/runtime-rpc/build/types/reflection-info';

import type { RpcOutputStream } from '@protobuf-ts/runtime-rpc/build/types/rpc-output-stream';

function objectifyRequests<O extends object>(
  original: RpcInputStream<O>
): RpcInputStream<O> {
  return {
    send: (...args) => original.send(...args),
    complete: (...args) => original.complete(...args),
  };
}

function objectifyResponses<O extends object>(
  original: RpcOutputStream<O>
): RpcOutputStream<O> {
  return {
    ...original,
    onMessage: (...args) => original.onMessage(...args),
    onError: (...args) => original.onError(...args),
    onComplete: (...args) => original.onComplete(...args),
    onNext: (...args) => original.onNext(...args),
  };
}

function objectifyError(e) {
  return { ...e, message: e.message, cause: e.cause, stack: e.stack };
}

type PublicPart<T> = { [K in keyof T]: T[K] }; //keyof only sees public properties

function validateAbortSignalType(abortSignal: AbortSignal) {
  if (abortSignal instanceof AbortSignal) {
    throw new Error(
      'You must not pass AbortSignal instance. Use objectified version (tshAbortSignal).'
    );
  }
}

export function getObjectifiedInterceptors(): RpcInterceptor[] {
  return [
    {
      interceptUnary(next, method, input, options) {
        validateAbortSignalType(options.abort);

        const output = next(method, input, options);
        // UnaryCall is a class with a private field,
        // so we can't assign an object to it.
        // The workaround is satisfying only the public part of it
        // and then casting to UnaryCall.
        const objectifiedUnaryCall: PublicPart<UnaryCall> = {
          ...output,
          response: output.response.catch(e => objectifyError(e)),
          then: (...args) =>
            output.then(...args).catch(e => {
              throw objectifyError(e);
            }),
        };
        return objectifiedUnaryCall as UnaryCall;
      },
      interceptClientStreaming(next, method, options) {
        validateAbortSignalType(options.abort);

        const output = next(method, options);
        const objectifiedClientStreamingCall: PublicPart<ClientStreamingCall> =
          {
            ...output,
            then: (...args) =>
              objectifiedClientStreamingCall.then(...args).catch(e => {
                throw objectifyError(e);
              }),
            requests: objectifyRequests(output.requests),
            response: output.response.catch(e => objectifyError(e)),
          };
        return objectifiedClientStreamingCall as ClientStreamingCall;
      },
      interceptServerStreaming(next, method, input, options) {
        validateAbortSignalType(options.abort);

        const output = next(method, input, options);
        const objectifiedServerStreamingCall: PublicPart<ServerStreamingCall> =
          {
            ...output,
            then: (...args) =>
              objectifiedServerStreamingCall.then(...args).catch(e => {
                throw objectifyError(e);
              }),
            responses: objectifyResponses(output.responses),
          };
        return objectifiedServerStreamingCall as ServerStreamingCall;
      },
      interceptDuplex(next, method, options) {
        validateAbortSignalType(options.abort);

        const output = next(method, options);
        const duplexStreamingCall: PublicPart<DuplexStreamingCall> = {
          ...output,
          requests: objectifyRequests(output.requests),
          responses: objectifyResponses(output.responses),
          then: (...args) =>
            duplexStreamingCall.then(...args).catch(e => {
              throw objectifyError(e);
            }),
        };
        return duplexStreamingCall as DuplexStreamingCall;
      },
    },
  ];
}

export function objectifyClient<T>(client: ServiceInfo): T {
  return client.methods.reduce<T>((previousValue, currentValue) => {
    const { localName } = currentValue;
    previousValue[localName] = (...args) => client[localName](...args);
    return previousValue;
  }, {} as T);
}
