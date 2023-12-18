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

import React from 'react';
import styled from 'styled-components';

import {
  ApplicationsIcon,
  DatabasesIcon,
  DesktopsIcon,
  KubernetesIcon,
  RolesIcon,
  ServersIcon,
} from 'design/SVGIcon';

import type { AccessRequestResource, Resource } from 'teleport/Assist/types';

interface ResourcesProps {
  resources: AccessRequestResource[];
}

const Container = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 5px;
`;

const Resource = styled.div`
  background: ${p => p.theme.colors.spotBackground[0]};
  border: 1px solid ${p => p.theme.colors.spotBackground[0]};
  padding: 2px 10px;
  border-radius: 7px;
  font-size: 13px;
  display: flex;
  align-items: center;
  cursor: pointer;
  position: relative;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[1]};
  }

  svg path {
    fill: ${p => p.theme.colors.text.slightlyMuted};
  }
`;

const ResourceLeafCluster = styled.div`
  background: ${p => p.theme.colors.spotBackground[0]};
  padding: 0 7px;
  border-radius: 5px;
  margin-left: -8px;
  height: inherit;
  margin-right: 7px;
  display: flex;
  align-items: center;
  color: ${p => p.theme.colors.text.slightlyMuted};

  svg {
    margin-right: 5px;
  }
`;

const ResourceName = styled.div`
  margin-left: 7px;

  & + ${ResourceLeafCluster} {
    margin-right: 10px;
  }
`;

export function Resources(props: ResourcesProps) {
  return (
    <Container>
      {props.resources.map((resource, index) => {
        const name =
          resource.type === 'node' ? resource.friendlyName : resource.id;

        return (
          <Resource key={index}>
            {getBadge(resource.type)}

            <ResourceName>{name}</ResourceName>
          </Resource>
        );
      })}
    </Container>
  );
}

interface RolesProps {
  roles: string[];
}

export function Roles(props: RolesProps) {
  return (
    <Container>
      {props.roles.map((role, index) => (
        <Resource key={index}>
          <RolesIcon size={14} />

          <ResourceName>{role}</ResourceName>
        </Resource>
      ))}
    </Container>
  );
}

function getBadge(type: string) {
  if (type === 'node') {
    return <ServersIcon size={14} />;
  }

  if (type === 'app') {
    return <ApplicationsIcon size={14} />;
  }

  if (type === 'kubernetes') {
    return <KubernetesIcon size={14} />;
  }

  if (type === 'desktop') {
    return <DesktopsIcon size={14} />;
  }

  if (type === 'database') {
    return <DatabasesIcon size={14} />;
  }

  return null;
}
