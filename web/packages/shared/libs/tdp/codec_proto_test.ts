/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import Codec, {
  ButtonState,
  MouseButton,
  ScrollAxis,
} from './codec';
import {MessageInfo, MessageType, readFieldOption, IMessageType, readMessageOption} from "@protobuf-ts/runtime";
import * as tdp from 'gen-proto-ts/teleport/desktop/tdp_pb';
//import { Any } from "@protobuf-ts/runtime";
import { Any } from 'gen-proto-ts/google/protobuf/any_pb';

const codec = new Codec();

test('encodes and decodes a mouse wheel event', () => {
  let msg = tdp.MouseWheel.create({axis: tdp.MouseWheelAxis.HORIZONTAL, delta: 3});
  let serialized = send([msg, tdp.MouseWheel]);

  let newMsg = tdp.MouseWheel.fromBinary(new Uint8Array(serialized));
  console.log(newMsg)
});

  function send<T extends object>(tup: [data: T, type: IMessageType<T>]): ArrayBufferLike {
    let data = tup[0], type = tup[1];
    //if (!this.transport) {
    //  this.logger.info('Transport is not ready, discarding message');
    //  return;
    //}

    let opt = readMessageOption(type, "tdp_type", tdp.TDPOptions);
    let msg = Any.pack(data, type);
    let envelope = tdp.TDPEnvelope.create({type: opt.tdpType, message: msg});

    return tdp.TDPEnvelope.toBinary(envelope).buffer
  }