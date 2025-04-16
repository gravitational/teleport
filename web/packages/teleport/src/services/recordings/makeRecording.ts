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

import cfg from 'teleport/config';
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
    createdDate: new Date(time),
    users: user,
    hostname: desktop_name,
    description,
    recordingType: 'desktop',
    playable: recorded,
  } as Recording;
}

function makeSshOrKubeRecording({
  participants,
  user,
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
  // SSH interactive/non-interactive and k8s interactive sessions user participants are in the participants field.
  let userParticipants = participants;
  // For Kubernetes sessions, put the full pod name as 'hostname'.
  if (proto === 'kube') {
    hostname = `${kubernetes_cluster}/${kubernetes_pod_namespace}/${kubernetes_pod_name}`;
    // For non-interactive k8s sessions the participant is the Teleport user running the command
    if (!interactive) userParticipants = [user];
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
    createdDate: new Date(time),
    users: userParticipants ? userParticipants.join(', ') : [],
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

function makeDatabaseRecording({
  time,
  session_start,
  session_stop,
  user,
  sid,
  db_service,
  db_protocol,
}) {
  const description = cfg.getPlayableDatabaseProtocols().includes(db_protocol)
    ? 'play'
    : 'non-interactive';
  let { duration, durationText } = formatDuration(session_start, session_stop);

  // Older database session recordings won't have start/stop fields. For those
  // recordings we set the duration to the smallest number so we can still
  // play them.
  // As a side effect, the progress bar does not work properly, showing always
  // as completed. Also, navigating through it won't work.
  if (duration === 0) {
    duration = 1;
    durationText = '-';
  }

  return {
    duration,
    durationText,
    sid,
    createdDate: new Date(time),
    users: user,
    hostname: db_service,
    description,
    recordingType: 'database',
    playable: description === 'play',
  } as Recording;
}

const disabledDescription = 'recording disabled';
