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

import React, { useEffect, useState } from 'react';
import { useParams, useHistory } from 'react-router';
import { Link } from 'react-router-dom';
import { mapResourceToViewItem } from 'shared/components/UnifiedResources/shared/viewItemsFactory';

import { ArrowBack } from 'design/Icon';

import useAttempt from 'shared/hooks/useAttemptNext';

import { useTeleport } from 'teleport';
import { FeatureBox } from 'teleport/components/Layout';
import { UnifiedResource } from 'teleport/services/agents';
import { ResourceInfo } from './ResourceInfo';
import Indicator from 'design/Indicator';

export function Resource() {
  const { resourceId, clusterId } = useParams<{
    clusterId: string;
    resourceId: string;
  }>();
  // const history = useHistory();

  const ctx = useTeleport();

  const [resource, setResource] = useState<UnifiedResource>();

  const { attempt: fetchResourceAttempt, run: runFetchResource } = useAttempt();

  useEffect(() => {
    runFetchResource(() =>
      ctx.resourceService
        .fetchUnifiedResources(clusterId, {
          query: `resource.metadata.name == "${resourceId}"`,
          sort: {
            fieldName: 'name',
            dir: 'ASC',
          },
          limit: 1,
        })
        .then(res => setResource(res.agents[0]))
    );
  }, []);

  return (
    <FeatureBox>
      {fetchResourceAttempt.status === 'processing' && <Indicator />}
      {fetchResourceAttempt.status === 'success' && (
        <ResourceInfo resource={resource} />
      )}
    </FeatureBox>
  );
}
