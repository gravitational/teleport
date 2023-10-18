import React from 'react';

import { ResourceLabel } from 'teleport/services/agents';

import { DbProtocol } from 'shared/services/databases';

export type UnifiedResourceApp = {
  kind: 'app';
  id: string;
  name: string;
  description: string;
  labels: ResourceLabel[];
  awsConsole: boolean;
  addrWithProtocol?: string;
  friendlyName?: string;
};

export interface UnifiedResourceDatabase {
  kind: 'db';
  name: string;
  description: string;
  type: string;
  protocol: DbProtocol;
  labels: ResourceLabel[];
}

export interface UnifiedResourceNode {
  kind: 'node';
  id: string;
  hostname: string;
  labels: ResourceLabel[];
  addr: string;
  tunnel: boolean;
  subKind?: string;
}

export interface UnifiedResourceKube {
  kind: 'kube_cluster';
  name: string;
  labels: ResourceLabel[];
}

export type UnifiedResourceDesktop = {
  kind: 'windows_desktop';
  os: 'windows' | 'linux' | 'darwin';
  name: string;
  addr: string;
  labels: ResourceLabel[];
};

export type UnifiedResourceWindowsDesktopService = {
  kind: 'windows_desktop_service';
  name: string;
  hostname: string;
  addr: string;
  labels: ResourceLabel[];
};

export type UnifiedResourceUserGroup = {
  kind: 'user_group';
  name: string;
  description: string;
  labels: ResourceLabel[];
};

export type UnifiedResourceUi = {
  ActionButton: React.JSX.Element;
};

export type SharedUnifiedResource = {
  resource:
    | UnifiedResourceApp
    | UnifiedResourceDatabase
    | UnifiedResourceNode
    | UnifiedResourceKube
    | UnifiedResourceDesktop
    | UnifiedResourceWindowsDesktopService
    | UnifiedResourceUserGroup;
  ui: UnifiedResourceUi;
};
