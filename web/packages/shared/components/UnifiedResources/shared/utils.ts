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
  UnifiedResourceItem,
  UnifiedResourceUi,
  UnifiedResourceNode,
  UnifiedResourceApp,
  UnifiedResourceDatabase,
  UnifiedResourceDesktop,
  UnifiedResourceKube,
  UnifiedResourceUserGroup,
  SharedUnifiedResource,
} from '../types';

export function makeUnifiedResourceItemNode(
  resource: UnifiedResourceNode,
  ui: UnifiedResourceUi
): UnifiedResourceItem {
  return {
    name: resource.hostname,
    SecondaryIcon: ServerIcon,
    primaryIconName: 'Server',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    type: formatNodeSubKind(resource.subKind),
    addr: resource.tunnel ? '' : resource.addr,
  };
}

export function makeUnifiedResourceItemDatabase(
  resource: UnifiedResourceDatabase,
  ui: UnifiedResourceUi
): UnifiedResourceItem {
  return {
    name: resource.name,
    SecondaryIcon: DatabaseIcon,
    primaryIconName: getDatabaseIconName(resource.protocol),
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    description: resource.description,
    type: resource.type,
  };
}

export function makeUnifiedResourceItemKube(
  resource: UnifiedResourceKube,
  ui: UnifiedResourceUi
): UnifiedResourceItem {
  return {
    name: resource.name,
    SecondaryIcon: KubernetesIcon,
    primaryIconName: 'Kube',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    type: 'Kubernetes',
  };
}

export function makeUnifiedResourceItemApp(
  resource: UnifiedResourceApp,
  ui: UnifiedResourceUi
): UnifiedResourceItem {
  return {
    name: resource.name,
    SecondaryIcon: ApplicationIcon,
    primaryIconName: guessAppIcon(resource),
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    type: resource.samlApp ? 'SAML Application' : 'Application',
    description: resource.samlApp ? '' : resource.description,
    addr: resource.addrWithProtocol,
  };
}

export function makeUnifiedResourceItemDesktop(
  resource: UnifiedResourceDesktop,
  ui: UnifiedResourceUi
): UnifiedResourceItem {
  return {
    name: resource.name,
    SecondaryIcon: DesktopIcon,
    primaryIconName: 'Windows',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    type: 'Windows',
    addr: resource.addr,
  };
}

export function makeUnifiedResourceItemUserGroup(
  resource: UnifiedResourceUserGroup,
  ui: UnifiedResourceUi
): UnifiedResourceItem {
  return {
    name: resource.name,
    SecondaryIcon: ServerIcon,
    primaryIconName: 'Server',
    ActionButton: ui.ActionButton,
    labels: resource.labels,
    type: 'User Group',
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

export function mapResourceToItem({ resource, ui }: SharedUnifiedResource) {
  switch (resource.kind) {
    case 'node':
      return makeUnifiedResourceItemNode(resource, ui);
    case 'db':
      return makeUnifiedResourceItemDatabase(resource, ui);
    case 'kube_cluster':
      return makeUnifiedResourceItemKube(resource, ui);
    case 'app':
      return makeUnifiedResourceItemApp(resource, ui);
    case 'windows_desktop':
      return makeUnifiedResourceItemDesktop(resource, ui);
    case 'user_group':
      return makeUnifiedResourceItemUserGroup(resource, ui);
  }
}
