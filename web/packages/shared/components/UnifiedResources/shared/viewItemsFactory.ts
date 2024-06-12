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

import {
  Application as ApplicationIcon,
  Database as DatabaseIcon,
  Kubernetes as KubernetesIcon,
  Server as ServerIcon,
  Desktop as DesktopIcon,
} from 'design/Icon';
import { ResourceIconName } from 'design/ResourceIcon';

import { DbProtocol } from 'shared/services/databases';
import { NodeSubKind } from 'shared/services';

import {
  UnifiedResourceViewItem,
  UnifiedResourceUi,
  UnifiedResourceNode,
  UnifiedResourceApp,
  UnifiedResourceDatabase,
  UnifiedResourceDesktop,
  UnifiedResourceKube,
  UnifiedResourceUserGroup,
  SharedUnifiedResource,
} from '../types';

export function makeUnifiedResourceViewItemNode(
  resource: UnifiedResourceNode,
  ui: UnifiedResourceUi
): UnifiedResourceViewItem {
  const nodeSubKind = formatNodeSubKind(resource.subKind);
  const addressIfNotTunnel = resource.tunnel ? '' : resource.addr;

  return {
    name: resource.hostname,
    SecondaryIcon: ServerIcon,
    primaryIconName: 'Server',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    cardViewProps: {
      primaryDesc: nodeSubKind,
      secondaryDesc: addressIfNotTunnel,
    },
    listViewProps: {
      resourceType: nodeSubKind,
      addr: addressIfNotTunnel,
    },
  };
}

export function makeUnifiedResourceViewItemDatabase(
  resource: UnifiedResourceDatabase,
  ui: UnifiedResourceUi
): UnifiedResourceViewItem {
  return {
    name: resource.name,
    SecondaryIcon: DatabaseIcon,
    primaryIconName: getDatabaseIconName(resource.protocol),
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    listViewProps: {
      description: resource.description,
      resourceType: resource.type,
    },
    cardViewProps: {
      primaryDesc: resource.type,
      secondaryDesc: resource.description,
    },
  };
}

export function makeUnifiedResourceViewItemKube(
  resource: UnifiedResourceKube,
  ui: UnifiedResourceUi
): UnifiedResourceViewItem {
  return {
    name: resource.name,
    SecondaryIcon: KubernetesIcon,
    primaryIconName: 'Kube',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    cardViewProps: {
      primaryDesc: 'Kubernetes',
    },
    listViewProps: {
      resourceType: 'Kubernetes',
    },
  };
}

export function makeUnifiedResourceViewItemApp(
  resource: UnifiedResourceApp,
  ui: UnifiedResourceUi
): UnifiedResourceViewItem {
  return {
    name: resource.friendlyName || resource.name,
    SecondaryIcon: ApplicationIcon,
    primaryIconName: guessAppIcon(resource),
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    cardViewProps: {
      primaryDesc: resource.description,
      secondaryDesc: resource.addrWithProtocol,
    },
    listViewProps: {
      resourceType: resource.samlApp ? 'SAML Application' : 'Application',
      description: resource.samlApp ? '' : resource.description,
      addr: resource.addrWithProtocol,
    },
  };
}

export function makeUnifiedResourceViewItemDesktop(
  resource: UnifiedResourceDesktop,
  ui: UnifiedResourceUi
): UnifiedResourceViewItem {
  return {
    name: resource.name,
    SecondaryIcon: DesktopIcon,
    primaryIconName: 'Windows',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    cardViewProps: {
      primaryDesc: 'Windows',
      secondaryDesc: resource.addr,
    },
    listViewProps: {
      resourceType: 'Windows',
      addr: resource.addr,
    },
  };
}

export function makeUnifiedResourceViewItemUserGroup(
  resource: UnifiedResourceUserGroup,
  ui: UnifiedResourceUi
): UnifiedResourceViewItem {
  return {
    name: resource.friendlyName || resource.name,
    SecondaryIcon: ServerIcon,
    primaryIconName: 'Server',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    cardViewProps: {},
    listViewProps: {
      resourceType: 'User Group',
    },
  };
}

function formatNodeSubKind(subKind: NodeSubKind): string {
  switch (subKind) {
    case 'openssh-ec2-ice':
    case 'openssh':
      return 'OpenSSH Server';

    default:
      return 'SSH Server';
  }
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

export function mapResourceToViewItem({ resource, ui }: SharedUnifiedResource) {
  switch (resource.kind) {
    case 'node':
      return makeUnifiedResourceViewItemNode(resource, ui);
    case 'db':
      return makeUnifiedResourceViewItemDatabase(resource, ui);
    case 'kube_cluster':
      return makeUnifiedResourceViewItemKube(resource, ui);
    case 'app':
      return makeUnifiedResourceViewItemApp(resource, ui);
    case 'windows_desktop':
      return makeUnifiedResourceViewItemDesktop(resource, ui);
    case 'user_group':
      return makeUnifiedResourceViewItemUserGroup(resource, ui);
  }
}
