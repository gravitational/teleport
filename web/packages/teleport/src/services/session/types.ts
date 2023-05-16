/*
Copyright 2019-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

export interface Participant {
  user: string;
}

export interface Session {
  kind: SessionKind;
  sid: string;
  namespace: string;
  login: string;
  created: Date;
  durationText: string;
  serverId: string;
  clusterId: string;
  parties: Participant[];
  addr: string;
  // resourceName depending on the "kind" field, is the name
  // of resource that the session is running in:
  //  - ssh: is referring to the hostname
  //  - k8s: is referring to the kubernetes cluster name
  //  - db: is referring to the database
  //  - desktop: is referring to the desktop
  //  - app: is referring to the app
  resourceName: string;
  // participantModes are the participant modes that are available to the user listing this session.
  participantModes: ParticipantMode[];
  // whether this session requires moderation or not. this is NOT if the session is currently being actively moderated
  moderated: boolean;
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
