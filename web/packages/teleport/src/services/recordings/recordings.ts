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

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import { makeRecording } from './makeRecording';
import type {
  RecordingsQuery,
  RecordingsResponse,
  RecordingType,
  SessionRecordingThumbnail,
} from './types';

const maxFetchLimit = 5000;

export default class RecordingsService {
  /**
   * @deprecated Use standalone `fetchRecordings` function defined below this class instead.
   */
  fetchRecordings(
    clusterId: string,
    params: RecordingsQuery
  ): Promise<RecordingsResponse> {
    return fetchRecordings({ clusterId, params });
  }

  /**
   * @deprecated Use `fetchSessionRecordingDuration` instead.
   */
  fetchRecordingDuration(
    clusterId: string,
    sessionId: string
  ): Promise<{ durationMs: number; recordingType: string }> {
    return fetchSessionRecordingDuration({
      clusterId,
      sessionId,
    });
  }
}

interface FetchSessionRecordingDurationVariables {
  clusterId: string;
  sessionId: string;
}

interface FetchSessionRecordingDurationResponse {
  durationMs: number;
  recordingType: RecordingType;
}

export async function fetchSessionRecordingDuration({
  clusterId,
  sessionId,
}: FetchSessionRecordingDurationVariables): Promise<FetchSessionRecordingDurationResponse> {
  const url = cfg.getSessionDurationUrl(clusterId, sessionId);
  const response = await api.get(url);

  if (!response) {
    throw new Error('Failed to fetch session recording duration');
  }

  return response;
}

interface FetchRecordingsVariables {
  clusterId: string;
  params: RecordingsQuery;
}

export async function fetchRecordings(
  { clusterId, params }: FetchRecordingsVariables,
  signal?: AbortSignal
): Promise<RecordingsResponse> {
  const start = params.from.toISOString();
  const end = params.to.toISOString();

  const url = cfg.getClusterEventsRecordingsUrl(clusterId, {
    start,
    end,
    limit: maxFetchLimit,
    startKey: params.startKey || undefined,
  });

  const json = await api.get(url, signal);

  const events = json.events || [];

  return { recordings: events.map(makeRecording), startKey: json.startKey };
}

interface FetchRecordingThumbnailVariables {
  clusterId: string;
  sessionId: string;
}

export async function fetchRecordingThumbnail(
  { clusterId, sessionId }: FetchRecordingThumbnailVariables,
  signal?: AbortSignal
): Promise<SessionRecordingThumbnail> {
  const url = cfg.getSessionRecordingThumbnailUrl(clusterId, sessionId);
  const response = await api.get(url, signal);

  if (!response) {
    throw new Error('Failed to fetch recording thumbnail');
  }

  return response as SessionRecordingThumbnail;
}

export const RECORDING_TYPES_WITH_THUMBNAILS: RecordingType[] = ['ssh', 'k8s'];
export const RECORDING_TYPES_WITH_METADATA: RecordingType[] = ['ssh', 'k8s'];

export const VALID_RECORDING_TYPES: RecordingType[] = [
  'ssh',
  'k8s',
  'desktop',
  'database',
];
