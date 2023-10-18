import React from 'react';
import { ResourceIconName } from 'design/ResourceIcon';

import {
  Icon,
  Application as ApplicationIcon,
  Database as DatabaseIcon,
  Kubernetes as KubernetesIcon,
  Server as ServerIcon,
  Desktop as DesktopIcon,
} from 'design/Icon';

import { DbProtocol } from 'shared/services/databases';

import {
  UnifiedResourceKube,
  UnifiedResourceNode,
  UnifiedResourceUi,
  UnifiedResourceDatabase,
  UnifiedResourceApp,
  UnifiedResourceWindowsDesktopService,
  UnifiedResourceUserGroup,
  UnifiedResourceDesktop,
} from './types';

export interface UnifiedResourceCard {
  name: string;
  description: {
    primary?: string;
    secondary?: string;
  };
  labels: {
    name: string;
    value: string;
  }[];
  primaryIconName: ResourceIconName;
  SecondaryIcon: typeof Icon;
  ActionButton: React.JSX.Element;
}

export function makeUnifiedResourceCardNode(
  resource: UnifiedResourceNode,
  ui: UnifiedResourceUi
): UnifiedResourceCard {
  return {
    name: resource.hostname,
    SecondaryIcon: ServerIcon,
    primaryIconName: 'Server',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    description: {
      primary: resource.subKind || 'SSH Server',
      secondary: resource.tunnel ? '' : resource.addr,
    },
  };
}

export function makeUnifiedResourceCardDatabase(
  resource: UnifiedResourceDatabase,
  ui: UnifiedResourceUi
): UnifiedResourceCard {
  return {
    name: resource.name,
    SecondaryIcon: DatabaseIcon,
    primaryIconName: getDatabaseIconName(resource.protocol),
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    description: { primary: resource.type, secondary: resource.description },
  };
}

export function makeUnifiedResourceCardKube(
  resource: UnifiedResourceKube,
  ui: UnifiedResourceUi
): UnifiedResourceCard {
  return {
    name: resource.name,
    SecondaryIcon: KubernetesIcon,
    primaryIconName: 'Kube',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    description: { primary: 'Kubernetes' },
  };
}

export function makeUnifiedResourceCardApp(
  resource: UnifiedResourceApp,
  ui: UnifiedResourceUi
): UnifiedResourceCard {
  return {
    name: resource.name,
    SecondaryIcon: ApplicationIcon,
    primaryIconName: guessAppIcon(resource),
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    description: {
      primary: resource.description,
      secondary: resource.addrWithProtocol,
    },
  };
}

export function makeUnifiedResourceCardDesktop(
  resource: UnifiedResourceDesktop,
  ui: UnifiedResourceUi
): UnifiedResourceCard {
  return {
    name: resource.name,
    SecondaryIcon: DesktopIcon,
    primaryIconName: 'Windows',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    description: { primary: 'Windows', secondary: resource.addr },
  };
}

export function makeUnifiedResourceCardWindowsDesktopService(
  resource: UnifiedResourceWindowsDesktopService,
  ui: UnifiedResourceUi
): UnifiedResourceCard {
  return {
    name: resource.name,
    SecondaryIcon: ServerIcon,
    primaryIconName: 'Windows',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    description: {},
  };
}

export function makeUnifiedResourceCardUserGroup(
  resource: UnifiedResourceUserGroup,
  ui: UnifiedResourceUi
): UnifiedResourceCard {
  return {
    name: resource.name,
    SecondaryIcon: ServerIcon,
    primaryIconName: 'Server',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    description: {},
  };
}

type GuessedAppType = 'Grafana' | 'Slack' | 'Jenkins' | 'Application' | 'Aws';

function guessAppIcon(app: UnifiedResourceApp): GuessedAppType {
  const { name, labels, friendlyName, awsConsole = false } = app;

  if (awsConsole) {
    return 'Aws';
  }

  if (
    name?.toLocaleLowerCase().includes('slack') ||
    friendlyName?.toLocaleLowerCase().includes('slack') ||
    labels?.some(l => `${l.name}:${l.value}` === 'icon:slack')
  ) {
    return 'Slack';
  }

  if (
    name?.toLocaleLowerCase().includes('grafana') ||
    friendlyName?.toLocaleLowerCase().includes('grafana') ||
    labels?.some(l => `${l.name}:${l.value}` === 'icon:grafana')
  ) {
    return 'Grafana';
  }

  if (
    name?.toLocaleLowerCase().includes('jenkins') ||
    friendlyName?.toLocaleLowerCase().includes('jenkins') ||
    labels?.some(l => `${l.name}:${l.value}` === 'icon:jenkins')
  ) {
    return 'Jenkins';
  }

  return 'Application';
}

function getDatabaseIconName(protocol: DbProtocol): ResourceIconName {
  switch (protocol) {
    case 'postgres':
      return 'Postgres';
    case 'mysql':
      return 'MysqlLarge';
    case 'mongodb':
      return 'Mongo';
    case 'cockroachdb':
      return 'Cockroach';
    case 'snowflake':
      return 'Snowflake';
    case 'dynamodb':
      return 'Dynamo';
    default:
      return 'Database';
  }
}
