/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React from 'react';
import { ResourceIcon } from 'design/ResourceIcon';

import { IntegrationKind, PluginKind } from 'teleport/services/integrations';

export function IntegrationIcon({
  type,
}: {
  type: PluginKind | IntegrationKind;
}) {
  if (type === 'okta') {
    return <ResourceIcon name="okta" mr={1} width="20px" height="20px" />;
  }
  if (type === 'aws-oidc') {
    return <ResourceIcon name="aws" mr={1} width="20px" height="20px" />;
  }

  return <ResourceIcon name="application" mr={1} width="20px" height="20px" />;
}

export function PanelIcon({ type }: { type: 'ec2' | 'rds' | 'eks' }) {
  if (type === 'ec2') {
    return <ResourceIcon name="ec2" mr={1} width="32px" height="32px" />;
  }
  if (type === 'rds') {
    return <ResourceIcon name="rds" mr={1} width="32px" height="32px" />;
  }
  if (type === 'eks') {
    return <ResourceIcon name="eks" mr={1} width="32px" height="32px" />;
  }
}
