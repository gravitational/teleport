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

export interface Participant {
  user: string;
  mode: string;
}

export interface Session {
  kind: SessionKind;
  sid: string;
  namespace?: string;
  login?: string;
  created?: Date;
  durationText?: string;
  serverId?: string;
  clusterId: string;
  parties?: Participant[];
  addr?: string;
  // resourceName depending on the "kind" field, is the name
  // of resource that the session is running in:
  //  - ssh: is referring to the hostname
  //  - k8s: is referring to the kubernetes cluster name
  //  - db: is referring to the database
  //  - desktop: is referring to the desktop
  //  - app: is referring to the app
  resourceName: string;
  // participantModes are the participant modes that are available to the user listing this session.
  participantModes?: ParticipantMode[];
  // whether this session requires moderation or not. this is NOT if the session is currently being actively moderated
  moderated?: boolean;
  // command is the command that was run to start this session.
  command?: string;
}

export type SessionMetadata = {
  kind: string;
  id: string;
  namespace: string;
  parties: any;
  terminal_params: {
    w: number;
    h: number;
  };
  login: string;
  created: string;
  last_active: string;
  server_id: string;
  server_hostname: string;
  server_hostport: number;
  server_addr: string;
  cluster_name: string;
  kubernetes_cluster_name: string;
  desktop_name: string;
  database_name: string;
  app_name: string;
  resourceName: string;
};

export type ParticipantMode = 'observer' | 'moderator' | 'peer';

export type ParticipantList = Record<string, Participant[]>;

export type SessionKind = 'ssh' | 'k8s' | 'db' | 'app' | 'desktop';
