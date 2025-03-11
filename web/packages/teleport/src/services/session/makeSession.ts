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

import { formatDistanceStrict } from 'date-fns';

import { Participant, Session, SessionKind } from './types';

const nameField: { [kind in SessionKind]: string } = {
  ssh: 'server_hostname',
  k8s: 'kubernetes_cluster_name',
  db: 'database_name',
  app: 'app_name',
  desktop: 'desktop_name',
};

export default function makeSession(json): Session {
  const {
    kind,
    id,
    namespace,
    login,
    created,
    server_id,
    cluster_name,
    server_addr,
    parties,
    participantModes,
    moderated,
    command,
  } = json;

  const createdDate = created ? new Date(created) : null;
  const durationText = createdDate
    ? formatDistanceStrict(new Date(), createdDate)
    : '';

  return {
    kind,
    sid: id,
    namespace,
    login,
    created: createdDate,
    durationText,
    serverId: server_id,
    resourceName: json[nameField[kind]],
    clusterId: cluster_name,
    parties: parties ? parties.map(p => makeParticipant(p)) : [],
    addr: server_addr ? server_addr.replace(PORT_REGEX, '') : '',
    participantModes: participantModes ?? [],
    moderated,
    command: command ?? '',
  };
}

export function makeParticipant(json): Participant {
  return {
    user: json.user,
    mode: json.mode,
  };
}

const PORT_REGEX = /:\d+$/;
