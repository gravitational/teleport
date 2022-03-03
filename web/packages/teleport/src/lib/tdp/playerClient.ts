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
  // TODO: MOVE = 'move'
}

export enum PlayerClientEvent {
  TOGGLE_PLAY_PAUSE = 'play/pause',
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
    this.socket?.send(JSON.stringify({ action: Action.TOGGLE_PLAY_PAUSE }));
    this.emit(PlayerClientEvent.TOGGLE_PLAY_PAUSE);
  }

  // Overrides Client implementation.
  processMessage(buffer: ArrayBuffer) {
    const json = JSON.parse(this.textDecoder.decode(buffer));

    if (json.message === 'end') {
      this.emit(PlayerClientEvent.SESSION_END);
    } else if (json.message === 'error') {
      this.emit(PlayerClientEvent.PLAYBACK_ERROR, new Error(json.errorText));
    } else {
      const ms = json.ms;
      this.emit(PlayerClientEvent.UPDATE_CURRENT_TIME, ms);
      super.processMessage(base64ToArrayBuffer(json.message));
    }
  }

  // Overrides Client implementation.
  handleClientScreenSpec(buffer: ArrayBuffer) {
    this.emit(
      TdpClientEvent.TDP_CLIENT_SCREEN_SPEC,
      this.codec.decodeClientScreenSpec(buffer)
    );
  }

  // Overrides Client implementation.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  handleMouseButton(buffer: ArrayBuffer) {
    // TODO
    return;
  }

  // Overrides Client implementation.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  handleMouseMove(buffer: ArrayBuffer) {
    // TODO
    return;
  }
}
