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

import { throttle } from 'shared/utils/highbar';
import { base64ToArrayBuffer } from 'shared/utils/base64';

import { StatusEnum } from 'teleport/lib/player';

import Client, { TdpClientEvent } from './client';
import { ClientScreenSpec } from './codec';

// we update the time every time we receive data, or
// at this interval (which ensures that the progress
// bar updates even when we aren't receiving data)
const PROGRESS_UPDATE_INTERVAL_MS = 50;

enum Action {
  TOGGLE_PLAY_PAUSE = 'play/pause',
  PLAY_SPEED = 'speed',
  SEEK = 'seek',
}

export class PlayerClient extends Client {
  private textDecoder = new TextDecoder();
  private setPlayerStatus: React.Dispatch<React.SetStateAction<StatusEnum>>;
  private setStatusText: React.Dispatch<React.SetStateAction<string>>;
  private _setTime: React.Dispatch<React.SetStateAction<number>>;
  private setTime: React.Dispatch<React.SetStateAction<number>>;

  private speed = 1.0;
  private paused = false;

  private lastPlayedTimestamp = 0;
  private skipTimeUpdatesUntil = null;
  private lastTimestamp = 0;
  private sendTimeUpdates = true;
  private lastUpdateTime = 0;
  private timeout = null;

  constructor({ url, setTime, setPlayerStatus, setStatusText }) {
    super(url);
    this.setPlayerStatus = setPlayerStatus;
    this.setStatusText = setStatusText;
    this._setTime = setTime;
    this.setTime = throttle(t => {
      // time updates are suspended when a user is dragging the slider to
      // a new position (it's very disruptive if we're updating the slider
      // position every few milliseconds while the user is trying to
      // reposition it)
      if (this.sendTimeUpdates) {
        this._setTime(t);
      }
      this.lastTimestamp = t;
      this.lastUpdateTime = Date.now();
    }, PROGRESS_UPDATE_INTERVAL_MS);
  }

  // Override so we can set player status.
  async connect(spec?: ClientScreenSpec) {
    await super.connect(spec);
    this.setPlayerStatus(StatusEnum.PLAYING);
  }

  scheduleNextUpdate(current: number) {
    this.timeout = setTimeout(() => {
      const delta = Date.now() - this.lastUpdateTime;
      const next = current + delta * this.speed;

      this.setTime(next);
      this.lastUpdateTime = Date.now();

      this.scheduleNextUpdate(next);
    }, PROGRESS_UPDATE_INTERVAL_MS);
  }

  cancelTimeUpdate() {
    if (this.timeout != null) {
      clearTimeout(this.timeout);
      this.timeout = null;
    }
  }

  suspendTimeUpdates() {
    this.sendTimeUpdates = false;
  }

  resumeTimeUpdates() {
    this.sendTimeUpdates = true;
  }

  // togglePlayPause toggles the playback system between "playing" and "paused" states.
  togglePlayPause() {
    this.paused = !this.paused;
    this.setPlayerStatus(this.paused ? StatusEnum.PAUSED : StatusEnum.PLAYING);
    if (this.paused) {
      this.cancelTimeUpdate();
    }

    this.lastUpdateTime = Date.now();

    if (this.isSeekingForward()) {
      const next = Math.max(this.skipTimeUpdatesUntil, this.lastTimestamp);
      this.scheduleNextUpdate(next);
    } else {
      this.scheduleNextUpdate(this.lastTimestamp);
    }

    this.send(JSON.stringify({ action: Action.TOGGLE_PLAY_PAUSE }));
  }

  // setPlaySpeed sets the playback speed of the recording.
  setPlaySpeed(speed: number) {
    this.speed = speed;
    this.send(JSON.stringify({ action: Action.PLAY_SPEED, speed }));
  }

  // Overrides Client implementation.
  async processMessage(buffer: ArrayBuffer): Promise<void> {
    const json = JSON.parse(this.textDecoder.decode(buffer));

    if (json.message === 'end') {
      this.setPlayerStatus(StatusEnum.COMPLETE);
    } else if (json.message === 'error') {
      this.setPlayerStatus(StatusEnum.ERROR);
      this.setStatusText(json.errorText);
    } else {
      this.cancelTimeUpdate();
      this.lastPlayedTimestamp = json.ms;
      this.lastUpdateTime = Date.now();
      this.setTime(json.ms);

      // clear seek state if we caught up to the seek point
      if (
        this.skipTimeUpdatesUntil !== null &&
        this.lastPlayedTimestamp >= this.skipTimeUpdatesUntil
      ) {
        this.skipTimeUpdatesUntil = null;
      }

      if (!this.isSeekingForward()) {
        this.cancelTimeUpdate();
      }

      // schedule the next time update, which ensures that
      // the progress bar continues to update even if this
      // section of the recording is idle time
      //
      // note: we don't schedule an update if we're currently
      // seeking forward in time, as we're trying to get there
      // as quickly as possible
      if (!this.paused && !this.isSeekingForward()) {
        this.scheduleNextUpdate(json.ms);
      }

      await super.processMessage(base64ToArrayBuffer(json.message));
    }
  }

  seekTo(pos: number) {
    this.cancelTimeUpdate();

    this.send(JSON.stringify({ action: Action.SEEK, pos }));

    this._setTime(pos);
    this.lastUpdateTime = Date.now();
    this.skipTimeUpdatesUntil = pos;

    if (pos < this.lastPlayedTimestamp) {
      // TODO: clear canvas
    } else if (!this.paused) {
      this.scheduleNextUpdate(pos);
    }
  }

  isSeekingForward(): boolean {
    return (
      this.skipTimeUpdatesUntil !== null &&
      this.skipTimeUpdatesUntil > this.lastPlayedTimestamp
    );
  }

  // Overrides Client implementation.
  handleClientScreenSpec(buffer: ArrayBuffer) {
    this.emit(
      TdpClientEvent.TDP_CLIENT_SCREEN_SPEC,
      this.codec.decodeClientScreenSpec(buffer)
    );
  }

  // Overrides Client implementation. This prevents the Client from sending
  // RDP response PDUs to the server during playback, which is unnecessary
  // and breaks the playback system.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  sendRdpResponsePDU(responseFrame: ArrayBuffer) {
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
