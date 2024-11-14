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

import React, { useEffect, PropsWithChildren, useState } from 'react';
import { useHistory, Link as InternalLink } from 'react-router-dom';
import { Indicator, Box, Alert, Flex, ButtonIcon, Text, Label } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';
import { useParams } from 'react-router';
import { ArrowLeft } from 'design/Icon';
import { HoverTooltip } from 'shared/components/ToolTip';
import { ResourceIcon } from 'design/ResourceIcon';
import { useAsync } from 'shared/hooks/useAsync';

import { FeatureBox } from 'teleport/components/Layout';
import {
  IntegrationKind,
  IntegrationStatusCode,
  PluginKind,
  PluginStatus,
} from 'teleport/services/integrations';
import cfg from 'teleport/config';

import { ListTasks } from './ListTasks';
import { ViewTask } from './ViewTask';

export function AwsOidcIntegrationPendingTasks({
  integrationName,
}: {
  integrationName: string;
}) {
  const [attempt, run, setAttempt] = useAsync(async () => {
    // TODO(update)
    // - fetch integration by integrationName
    // - THEN fetch pending tasks
    return await Promise.resolve([
      {
        issueDetail: 'Failed to auto-enroll resources',
        type: 'ec2',
      },
      {
        issueDetail: `EC2 instance's OS is not compatible with \
        teleport (eg windows) lorem ipsum dolores testing long \
        detail george washington is the first president of the \
        united states`,
        type: 'ec2',
      },
    ]);
  });

  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [viewingTask, setViewingTask] = useState();

  useEffect(() => {
    // Permission check??

    run();
    // Fetch status details for aws integration
    // run(() => pluginsService.fetchPluginStatus(name).then(setStatus));
  }, []);
  return (
    <Flex css={{ flex: 1 }}>
      <Box
        p={4}
        css={`
          overflow: auto;
          position: relative;
          right: 0;
          width: 100%;
        `}
      >
        <Flex alignItems="center" mb={3} justifyContent="space-between">
          <Flex alignItems="center" mr={3}>
            <HoverTooltip tipContent="Back to Status Page">
              <ButtonIcon
                as={InternalLink}
                to={cfg.getIntegrationStatusRoute(
                  IntegrationKind.AwsOidc,
                  integrationName
                )}
                mr={2}
                ml={'-8px'} // TODO required??
              >
                <ArrowLeft size="medium" />
              </ButtonIcon>
            </HoverTooltip>
            <Text typography="h1">{integrationName} </Text>
          </Flex>
        </Flex>
        {attempt.status === 'error' && (
          <Alert mt={3}>
            <Flex alignItems="center">
              <Text>{attempt.statusText}</Text>
            </Flex>
          </Alert>
        )}
        {attempt.status === 'processing' && (
          <Flex justifyContent="center">
            <Indicator />
          </Flex>
        )}
        {attempt.status === 'success' && (
          <>
            <ListTasks />
            {attempt.data.length > 0 && (
              <ViewTask
              // // key creates a new component instance when rule changes
              // // instead of updating the mounted component
              // key={viewingRule?.object.metadata.name}
              />
            )}
          </>
        )}
      </Box>
    </Flex>
  );
}
