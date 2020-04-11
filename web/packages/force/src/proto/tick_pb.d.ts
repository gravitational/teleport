// package: proto
// file: tick.proto

import * as jspb from "google-protobuf";

export class Tick extends jspb.Message {
  getTime(): number;
  setTime(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Tick.AsObject;
  static toObject(includeInstance: boolean, msg: Tick): Tick.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Tick, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Tick;
  static deserializeBinaryFromReader(message: Tick, reader: jspb.BinaryReader): Tick;
}

export namespace Tick {
  export type AsObject = {
    time: number,
  }
}

export class TickRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): TickRequest.AsObject;
  static toObject(includeInstance: boolean, msg: TickRequest): TickRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: TickRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): TickRequest;
  static deserializeBinaryFromReader(message: TickRequest, reader: jspb.BinaryReader): TickRequest;
}

export namespace TickRequest {
  export type AsObject = {
  }
}

