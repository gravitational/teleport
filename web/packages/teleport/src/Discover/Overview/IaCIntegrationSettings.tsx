/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { Link as RouterLink } from 'react-router-dom';

import { Flex, Indicator, Text } from 'design';
import { Danger } from 'design/Alert';
import ButtonIcon from 'design/ButtonIcon';
import { ArrowLeft } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { useNoMinWidth } from 'teleport/Main';
import {
  IntegrationKind,
  integrationService,
  IntegrationWithSummary,
} from 'teleport/services/integrations';

import { SettingsTab } from './SettingsTab';
import { SettingsTabContainer } from './SettingsTabContainer';

export function IaCIntegrationSettings() {
  const { name, type } = useParams<{ name: string; type: string }>();

  const {
    data: stats,
    error,
    isLoading,
    isError,
  } = useQuery<IntegrationWithSummary>({
    queryKey: ['integrationStats', name],
    queryFn: () => integrationService.fetchIntegrationStats(name),
    enabled: !!name,
  });

  useNoMinWidth();

  const overviewPath = cfg.getIaCIntegrationRoute(
    type as IntegrationKind,
    name
  );
  const header = (
    <Flex alignItems="center" mb={3}>
      <HoverTooltip placement="bottom" tipContent="Back to Overview">
        <ButtonIcon as={RouterLink} to={overviewPath} mr={2}>
          <ArrowLeft size="medium" />
        </ButtonIcon>
      </HoverTooltip>
      <Text bold fontSize={6} mr={2}>
        {name}: Edit configuration
      </Text>
    </Flex>
  );

  if (isLoading) {
    return (
      <FeatureBox px={0}>
        <SettingsTabContainer>
          {header}
          <Flex justifyContent="center" mt={6}>
            <Indicator delay="long" />
          </Flex>
        </SettingsTabContainer>
      </FeatureBox>
    );
  }

  if (isError) {
    return (
      <FeatureBox px={0}>
        <SettingsTabContainer>
          {header}
          <Danger>
            {error?.message || 'Failed to load integration stats'}
          </Danger>
        </SettingsTabContainer>
      </FeatureBox>
    );
  }

  return (
    <FeatureBox px={0} hideBottomSpacing>
      <SettingsTab stats={stats} header={header} />
    </FeatureBox>
  );
}
