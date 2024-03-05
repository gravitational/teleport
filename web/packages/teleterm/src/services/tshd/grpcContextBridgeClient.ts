import {
  RpcInputStream,
  UnaryCall,
  ClientStreamingCall,
  ServerStreamingCall,
  DuplexStreamingCall,
  RpcInterceptor,
  RpcOutputStream,
  ServiceInfo,
  RpcError,
} from '@protobuf-ts/runtime-rpc';

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
    onError: errorCallback =>
      original.onError(e => errorCallback(objectifyError(e) as Error)),
    onComplete: (...args) => original.onComplete(...args),
    onNext: (...args) => original.onNext(...args),
  };
}

function objectifyError(error: unknown): object {
  const e = error as RpcError;
  return {
    ...e,
    message: e.message,
    cause: e.cause,
    stack: e.stack,
    code: e.code,
    isResolvableWithRelogin: e.meta['is-resolvable-with-relogin'] === '1',
  };
}

type PublicProperties<T> = { [K in keyof T]: T[K] }; //keyof only sees public properties

function validateAbortSignalType(
  abortSignal: AbortSignal | ObjectifiedAbortSignal
) {
  if (abortSignal && abortSignal['canBePassedThroughContextBridge'] !== true) {
    throw new Error(
      'You must not pass AbortSignal instance. Use objectified version (ObjectifedAbortSignal).'
    );
  }
}

async function objectifyPromiseRejection<TResult>(
  p: Promise<TResult>
): Promise<TResult> {
  try {
    return await p;
  } catch (e) {
    throw objectifyError(e);
  }
}

function objectifyThenRejection<TResult>(
  originalThen: Promise<TResult>['then']
): Promise<TResult>['then'] {
  return (onFulfilled, onRejected) => {
    // If onRejected callback is provided, then it will handle the rejection.
    if (onRejected) {
      return originalThen(onFulfilled, reason =>
        onRejected(objectifyError(reason))
      );
    }
    return objectifyPromiseRejection(originalThen(onFulfilled));
  };
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
        const objectifiedUnaryCall: PublicProperties<UnaryCall> = {
          method: output.method,
          request: output.request,
          requestHeaders: output.requestHeaders,
          status: objectifyPromiseRejection(output.status),
          trailers: objectifyPromiseRejection(output.trailers),
          headers: objectifyPromiseRejection(output.headers),
          response: objectifyPromiseRejection(output.response),
          // then is a method on UnaryClass,
          // we have to set correct "this" before passing it further.
          then: objectifyThenRejection(output.then.bind(output)),
        };
        return { ...output, ...objectifiedUnaryCall } as UnaryCall;
      },
      interceptClientStreaming(next, method, options) {
        validateAbortSignalType(options.abort);

        const output = next(method, options);
        const objectifiedClientStreamingCall: PublicProperties<ClientStreamingCall> =
          {
            method: output.method,
            requestHeaders: output.requestHeaders,
            status: objectifyPromiseRejection(output.status),
            trailers: objectifyPromiseRejection(output.trailers),
            headers: objectifyPromiseRejection(output.headers),
            response: objectifyPromiseRejection(output.response),
            requests: objectifyRequests(output.requests),
            then: objectifyThenRejection(output.then.bind(output)),
          };
        return {
          ...output,
          ...objectifiedClientStreamingCall,
        } as ClientStreamingCall;
      },
      interceptServerStreaming(next, method, input, options) {
        validateAbortSignalType(options.abort);

        const output = next(method, input, options);
        const objectifiedServerStreamingCall: PublicProperties<ServerStreamingCall> =
          {
            method: output.method,
            request: output.request,
            requestHeaders: output.requestHeaders,
            status: objectifyPromiseRejection(output.status),
            trailers: objectifyPromiseRejection(output.trailers),
            headers: objectifyPromiseRejection(output.headers),
            responses: objectifyResponses(output.responses),
            then: objectifyThenRejection(output.then.bind(output)),
          };
        return {
          ...output,
          ...objectifiedServerStreamingCall,
        } as ServerStreamingCall;
      },
      interceptDuplex(next, method, options) {
        validateAbortSignalType(options.abort);

        const output = next(method, options);
        const duplexStreamingCall: PublicProperties<DuplexStreamingCall> = {
          method: output.method,
          requestHeaders: output.requestHeaders,
          status: objectifyPromiseRejection(output.status),
          trailers: objectifyPromiseRejection(output.trailers),
          headers: objectifyPromiseRejection(output.headers),
          requests: objectifyRequests(output.requests),
          responses: objectifyResponses(output.responses),
          then: objectifyThenRejection(output.then.bind(output)),
        };
        return { ...output, ...duplexStreamingCall } as DuplexStreamingCall;
      },
    },
  ];
}

export function objectifyClient<T>(client: T & ServiceInfo): T {
  return client.methods.reduce<T>((objectifiedClient, method) => {
    const { localName } = method;
    objectifiedClient[localName] = (...args) => client[localName](...args);
    return objectifiedClient;
  }, {} as T);
}

export type ObjectifiedAbortSignal = AbortSignal & {
  canBePassedThroughContextBridge: true;
};

export function objectifyAbortSignal(a: AbortSignal): ObjectifiedAbortSignal {
  return {
    canBePassedThroughContextBridge: true,
    onabort: (...args) => a.onabort(...args),
    throwIfAborted: () => a.throwIfAborted(),
    // getters allow reading the fresh value of class fields
    get reason() {
      return a.reason;
    },
    get aborted() {
      return a.aborted;
    },
    dispatchEvent: (...args) => a.dispatchEvent(...args),
    addEventListener: (type, listener, options) =>
      a.addEventListener(type, listener, options),
    removeEventListener: (type, listener, options) =>
      a.removeEventListener(type, listener, options),
    eventListeners: (...args) => a.eventListeners(...args),
    removeAllListeners: (...args) => a.removeAllListeners(...args),
  };
}
