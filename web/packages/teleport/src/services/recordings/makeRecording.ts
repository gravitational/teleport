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

import { differenceInMilliseconds, formatDistanceStrict } from 'date-fns';

import { eventCodes } from 'teleport/services/audit';

import { Recording } from './types';

// Takes in json objects built by SessionEnd and WindowsDesktopSessionEnd as defined in teleport/api/types/events/events.proto.
export function makeRecording(event: any): Recording {
  if (event.code === eventCodes.DESKTOP_SESSION_ENDED) {
    return makeDesktopRecording(event);
  } else if (event.code === eventCodes.DATABASE_SESSION_ENDED) {
    return makeDatabaseRecording(event);
  } else {
    return makeSshOrKubeRecording(event);
  }
}

function makeDesktopRecording({
  time,
  session_start,
  session_stop,
  user,
  sid,
  desktop_name,
  recorded,
}) {
  const { duration, durationText } = formatDuration(
    session_start,
    session_stop
  );

  let description = recorded ? 'play' : disabledDescription;

  return {
    duration,
    durationText,
    sid,
    createdDate: time,
    users: user,
    hostname: desktop_name,
    description,
    recordingType: 'desktop',
    playable: recorded,
  } as Recording;
}

function makeDatabaseRecording({
  session_start,
  session_stop,
  time,
  user,
  sid,
  db_user,
  db_service,
  db_name,
  db_protocol,
}) {
  let { duration, durationText } = formatDuration(
      session_start,
      session_stop
  );
  if (duration === 0) {
    return {
      sid,
      createdDate: time,
      users: user,
      hostname: `${db_user}@${db_service}/${db_name}`,
      description: 'non-interactive',
      playable: false,
      recordingType: 'database',
    } as Recording;
  }

  let playable = db_protocol === 'postgres';
  let description = playable ? 'play' : 'non-interactive';
  return {
    duration,
    durationText,
    description,
    sid,
    createdDate: time,
    users: user,
    hostname: `${db_user}@${db_service}/${db_name}`,
    playable,
    recordingType: 'database',
  } as Recording;
}

function makeSshOrKubeRecording({
  participants,
  time,
  session_start,
  session_stop,
  server_hostname,
  interactive,
  session_recording = 'on',
  sid,
  proto = '',
  kubernetes_cluster = '',
  kubernetes_pod_namespace = '',
  kubernetes_pod_name = '',
}): Recording {
  const { duration, durationText } = formatDuration(
    session_start,
    session_stop
  );

  let hostname = server_hostname || 'N/A';
  // For Kubernetes sessions, put the full pod name as 'hostname'.
  if (proto === 'kube') {
    hostname = `${kubernetes_cluster}/${kubernetes_pod_namespace}/${kubernetes_pod_name}`;
  }

  // Description set to play for interactive so users can search by "play".
  let description = interactive ? 'play' : 'non-interactive';
  let playable = session_recording === 'off' ? false : interactive;
  if (session_recording === 'off') {
    description = disabledDescription;
  }

  return {
    duration,
    durationText,
    sid,
    createdDate: time,
    users: participants ? participants.join(', ') : [],
    hostname,
    description,
    recordingType: kubernetes_cluster ? 'k8s' : 'ssh',
    playable,
  } as Recording;
}

function formatDuration(startDateString: string, stopDateString: string) {
  let durationText = '';
  let duration = 0;
  if (startDateString && stopDateString) {
    const start = new Date(startDateString);
    const end = new Date(stopDateString);

    duration = differenceInMilliseconds(end, start);
    durationText = formatDistanceStrict(start, end);
  }

  return { duration, durationText };
}

const disabledDescription = 'recording disabled';
