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

import { EventEmitter } from 'events';

import { Logger } from 'design/logger';
import {
  ClientScreenSpec,
  selectDirectoryInBrowser,
  TdpbCodec,
  TdpClient,
  TdpClientEvent,
} from 'shared/libs/tdp';
import { base64ToArrayBuffer } from 'shared/utils/base64';

import { AuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket';
import { StatusEnum } from 'teleport/lib/player';

import { adaptWebSocketToTdpTransport } from './webSocketTransportAdapter';

enum Action {
  TOGGLE_PLAY_PAUSE = 'play/pause',
  PLAY_SPEED = 'speed',
  SEEK = 'seek',
}

enum PlayerClientEvent {
  TIME_UPDATE = 'time update',
  PLAYER_STATUS = 'player status',
}

/**
 * PlayerTimeAnchor is an authoritative playback position. Anchors are only emitted
 * when the client learns the real position (a server message or a seek) - consumers
 * are expected to interpolate between anchors using the speed and paused flags.
 */
export interface PlayerTimeAnchor {
  ms: number;
  speed: number;
  paused: boolean;
}

type PlayerEventMap = {
  [PlayerClientEvent.TIME_UPDATE]: [PlayerTimeAnchor];
  [PlayerClientEvent.PLAYER_STATUS]: [StatusEnum];
};

export class PlayerClient extends TdpClient {
  private textDecoder = new TextDecoder();
  private playerEvents = new EventEmitter<PlayerEventMap>();

  private speed = 1.0;
  private paused = false;

  private lastPlayedTimestamp = 0;
  private skipTimeUpdatesUntil: number | null = null;
  private tdpbCodec = new TdpbCodec();

  constructor({ url }: { url: string }) {
    super(
      signal =>
        adaptWebSocketToTdpTransport(new AuthenticatedWebSocket(url), signal),
      selectDirectoryInBrowser,
      new Logger('TDPClient')
    );
  }

  onTimeUpdate = (listener: (anchor: PlayerTimeAnchor) => void) => {
    this.playerEvents.on(PlayerClientEvent.TIME_UPDATE, listener);
    return () => this.playerEvents.off(PlayerClientEvent.TIME_UPDATE, listener);
  };

  onPlayerStatus = (listener: (status: StatusEnum) => void) => {
    this.playerEvents.on(PlayerClientEvent.PLAYER_STATUS, listener);
    return () =>
      this.playerEvents.off(PlayerClientEvent.PLAYER_STATUS, listener);
  };

  private emitTimeAnchor(ms: number) {
    this.playerEvents.emit(PlayerClientEvent.TIME_UPDATE, {
      ms,
      speed: this.speed,
      paused: this.paused,
    });
  }

  // togglePlayPause toggles the playback system between "playing" and "paused" states.
  togglePlayPause() {
    this.paused = !this.paused;
    this.playerEvents.emit(
      PlayerClientEvent.PLAYER_STATUS,
      this.paused ? StatusEnum.PAUSED : StatusEnum.PLAYING
    );
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
      this.playerEvents.emit(
        PlayerClientEvent.PLAYER_STATUS,
        StatusEnum.COMPLETE
      );
    } else if (json.message === 'error') {
      this.emit(TdpClientEvent.ERROR, new Error(json.errorText));
    } else {
      this.lastPlayedTimestamp = json.ms;

      // clear seek state if we caught up to the seek point
      if (
        this.skipTimeUpdatesUntil !== null &&
        this.lastPlayedTimestamp >= this.skipTimeUpdatesUntil
      ) {
        this.skipTimeUpdatesUntil = null;
      }

      // no time anchors while catching up to a seek (seekTo already emitted the target)
      if (this.skipTimeUpdatesUntil === null) {
        this.emitTimeAnchor(json.ms);
      }

      // Handle TDPB recordings by switching to the TDPB codec
      if (json.tdpb_message) {
        await super.processMessage(
          base64ToArrayBuffer(json.tdpb_message),
          this.tdpbCodec
        );
      } else {
        // Handle TDP recordings
        await super.processMessage(base64ToArrayBuffer(json.message));
      }
    }
  }

  seekTo(pos: number) {
    this.send(JSON.stringify({ action: Action.SEEK, pos }));

    this.skipTimeUpdatesUntil = pos;
    this.emitTimeAnchor(pos);
  }

  // Overrides Client implementation.
  handleClientScreenSpec(spec: ClientScreenSpec) {
    this.emit(TdpClientEvent.TDP_CLIENT_SCREEN_SPEC, spec);
  }

  // Overrides Client implementation. This prevents the Client from sending
  // RDP response PDUs to the server during playback, which is unnecessary
  // and breaks the playback system.
  sendRdpResponsePdu() {
    return;
  }

  // Overrides Client implementation.
  handleMouseButton() {
    return;
  }

  // Overrides Client implementation.
  handleMouseMove() {
    return;
  }
}
