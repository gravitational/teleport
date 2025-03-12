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

import { Link } from 'react-router-dom';

import { Button } from 'design';

import cfg from 'teleport/config';
import { SearchResource } from 'teleport/Discover/SelectResource';

export default function AgentButtonAdd(props: Props) {
  const { canCreate, isLeafCluster, onClick, agent, beginsWithVowel } = props;
  const disabled = isLeafCluster || !canCreate;

  // Don't render button if it's disabled and feature hiding is enabled.
  const hidden = disabled && cfg.hideInaccessibleFeatures;

  let title = '';
  if (!canCreate) {
    if (agent === SearchResource.UNIFIED_RESOURCE) {
      title = `You do not have access to add resources.`;
    } else {
      title = `You do not have access to add ${
        beginsWithVowel ? 'an' : 'a'
      } ${agent}`;
    }
  }

  if (isLeafCluster) {
    if (agent === SearchResource.UNIFIED_RESOURCE) {
      title = `Adding resources to a leaf cluster is not supported.`;
    } else {
      title = `Adding ${
        beginsWithVowel ? 'an' : 'a'
      } ${agent} to a leaf cluster is not supported`;
    }
  }

  if (hidden) {
    return null;
  }

  return (
    <Link
      to={{
        pathname: `${cfg.routes.root}/discover`,
        state: { entity: agent !== 'unified_resource' ? agent : null },
      }}
      style={{ textDecoration: 'none' }}
    >
      <Button
        intent="primary"
        fill="border"
        title={title}
        disabled={disabled}
        width="240px"
        onClick={onClick}
      >
        {agent === 'unified_resource' ? 'Enroll New Resource' : `Add ${agent}`}
      </Button>
    </Link>
  );
}

export type Props = {
  isLeafCluster: boolean;
  canCreate: boolean;
  onClick?: () => void;
  agent: SearchResource;
  beginsWithVowel: boolean;
};
