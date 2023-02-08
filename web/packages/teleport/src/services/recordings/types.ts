/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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

export type RecordingType = 'ssh' | 'desktop' | 'k8s';

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
};
