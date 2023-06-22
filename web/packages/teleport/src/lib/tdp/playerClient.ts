// Copyright 2021 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { base64ToArrayBuffer } from 'shared/utils/base64';

import Client, { TdpClientEvent } from './client';

enum Action {
  TOGGLE_PLAY_PAUSE = 'play/pause',
  PLAY_SPEED = 'speed',
  // TODO: MOVE = 'move'
}

export enum PlayerClientEvent {
  TOGGLE_PLAY_PAUSE = 'play/pause',
  PLAY_SPEED = 'speed',
  UPDATE_CURRENT_TIME = 'time',
  SESSION_END = 'end',
  PLAYBACK_ERROR = 'playback error',
}

export class PlayerClient extends Client {
  textDecoder = new TextDecoder();

  constructor(socketAddr: string) {
    super(socketAddr);
  }

  // togglePlayPause toggles the playback system between "playing" and "paused" states.
  togglePlayPause() {
    this.send(JSON.stringify({ action: Action.TOGGLE_PLAY_PAUSE }));
    this.emit(PlayerClientEvent.TOGGLE_PLAY_PAUSE);
  }

  // setPlaySpeed sets the playback speed of the recording.
  setPlaySpeed(speed: number) {
    this.send(JSON.stringify({ action: Action.PLAY_SPEED, speed }));
    this.emit(PlayerClientEvent.PLAY_SPEED, speed);
  }

  // Overrides Client implementation.
  async processMessage(buffer: ArrayBuffer): Promise<void> {
    const json = JSON.parse(this.textDecoder.decode(buffer));

    if (json.message === 'end') {
      this.emit(PlayerClientEvent.SESSION_END);
    } else if (json.message === 'error') {
      this.emit(PlayerClientEvent.PLAYBACK_ERROR, new Error(json.errorText));
    } else {
      const ms = json.ms;
      this.emit(PlayerClientEvent.UPDATE_CURRENT_TIME, ms);
      await super.processMessage(base64ToArrayBuffer(json.message));
    }
  }

  // Overrides Client implementation. This prevents the Client from sending
  handleClientScreenSpec(buffer: ArrayBuffer) {
    const spec = this.codec.decodeClientScreenSpec(buffer);
    this.initFastPathProcessor(spec);
    this.emit(TdpClientEvent.TDP_CLIENT_SCREEN_SPEC, spec);
  }

  // Overrides Client implementation. This prevents the Client from sending
  // RDP response PDUs to the server during playback, which is unnecessary
  // and breaks the playback system.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  sendRDPResponsePDU(responseFrame: ArrayBuffer) {
    return;
  }

  // Overrides Client implementation.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  handleMouseButton(buffer: ArrayBuffer) {
    return;
  }

  // Overrides Client implementation.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  handleMouseMove(buffer: ArrayBuffer) {
    return;
  }
}
