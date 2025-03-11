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

export type RecordingsQuery = {
  from: Date;
  to: Date;
  limit?: number;
  startKey?: string;
};

export type RecordingsResponse = {
  recordings: Recording[];
  startKey: string;
};

export type RecordingType = 'ssh' | 'desktop' | 'k8s' | 'database';

export type Recording = {
  duration: number;
  durationText: string;
  sid: string;
  createdDate: Date;
  users: string;
  hostname: string;
  description: string;
  recordingType: RecordingType;
  playable: boolean;
  summary: string;
};
