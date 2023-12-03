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
