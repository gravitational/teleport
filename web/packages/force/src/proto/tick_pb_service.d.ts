// package: proto
// file: tick.proto

import * as tick_pb from './tick_pb';
import { grpc } from '@improbable-eng/grpc-web';

type TickServiceSubscribe = {
  readonly methodName: string;
  readonly service: typeof TickService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof tick_pb.TickRequest;
  readonly responseType: typeof tick_pb.Tick;
};

type TickServiceNow = {
  readonly methodName: string;
  readonly service: typeof TickService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof tick_pb.TickRequest;
  readonly responseType: typeof tick_pb.Tick;
};

export class TickService {
  static readonly serviceName: string;
  static readonly Subscribe: TickServiceSubscribe;
  static readonly Now: TickServiceNow;
}

export type ServiceError = {
  message: string;
  code: number;
  metadata: grpc.Metadata;
};
export type Status = { details: string; code: number; metadata: grpc.Metadata };

interface UnaryResponse {
  cancel(): void;
}
interface ResponseStream<T> {
  cancel(): void;
  on(type: 'data', handler: (message: T) => void): ResponseStream<T>;
  on(type: 'end', handler: (status?: Status) => void): ResponseStream<T>;
  on(type: 'status', handler: (status: Status) => void): ResponseStream<T>;
}
interface RequestStream<T> {
  write(message: T): RequestStream<T>;
  end(): void;
  cancel(): void;
  on(type: 'end', handler: (status?: Status) => void): RequestStream<T>;
  on(type: 'status', handler: (status: Status) => void): RequestStream<T>;
}
interface BidirectionalStream<ReqT, ResT> {
  write(message: ReqT): BidirectionalStream<ReqT, ResT>;
  end(): void;
  cancel(): void;
  on(
    type: 'data',
    handler: (message: ResT) => void
  ): BidirectionalStream<ReqT, ResT>;
  on(
    type: 'end',
    handler: (status?: Status) => void
  ): BidirectionalStream<ReqT, ResT>;
  on(
    type: 'status',
    handler: (status: Status) => void
  ): BidirectionalStream<ReqT, ResT>;
}

export class TickServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  subscribe(
    requestMessage: tick_pb.TickRequest,
    metadata?: grpc.Metadata
  ): ResponseStream<tick_pb.Tick>;
  now(
    requestMessage: tick_pb.TickRequest,
    metadata: grpc.Metadata,
    callback: (
      error: ServiceError | null,
      responseMessage: tick_pb.Tick | null
    ) => void
  ): UnaryResponse;
  now(
    requestMessage: tick_pb.TickRequest,
    callback: (
      error: ServiceError | null,
      responseMessage: tick_pb.Tick | null
    ) => void
  ): UnaryResponse;
}
