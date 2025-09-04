/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useQueries } from '@tanstack/react-query';
import { useHistory, useParams } from 'react-router';
import { Link as InternalLink } from 'react-router-dom';
import { useTheme } from 'styled-components';

import { ButtonIcon, Flex, Label, Link, MenuItem, Text } from 'design';
import * as Alerts from 'design/Alert';
import * as Icons from 'design/Icon';
import { ArrowLeft } from 'design/Icon';
import { ShimmerBox } from 'design/ShimmerBox';
import { HoverTooltip } from 'design/Tooltip';
import { MenuButton } from 'shared/components/MenuAction';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';

import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { Guide } from 'teleport/Integrations/Enroll/AwsConsole/Guide';
import { getStatusAndLabel } from 'teleport/Integrations/helpers';
import {
  IntegrationOperations,
  useIntegrationOperation,
} from 'teleport/Integrations/Operations';
import type { EditableIntegrationFields } from 'teleport/Integrations/Operations/useIntegrationOperation';
import { AwsOidcHeader } from 'teleport/Integrations/status/AwsOidc/AwsOidcHeader';
import { ConsoleCardEnrolled } from 'teleport/Integrations/status/AwsOidc/Cards/ConsoleCard';
import {
  IntegrationAwsRa,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';

export function AwsRaDashboard() {
  const { name } = useParams<{
    type: IntegrationKind;
    name: string;
  }>();

  const results = useQueries({
    queries: [
      {
        queryKey: ['integration'],
        gcTime: 0, // this prevents other pages from seeing cached data
        queryFn: () =>
          integrationService
            .fetchIntegration<IntegrationAwsRa>(name)
            .then(data => data),
      },
      {
        queryKey: ['stats'],
        gcTime: 0, // this prevents other pages from seeing cached data
        queryFn: () =>
          integrationService
            .fetchIntegrationStats(name)
            .then(data => data.rolesAnywhereProfileSync),
      },
    ],
  });

  const [integrationResp, statsResp] = results;
  if (results.some(r => r.status === 'error')) {
    return (
      <Alerts.Danger
        details={integrationResp?.error?.message || statsResp?.error?.message}
      >
        Error: {integrationResp?.error?.name || statsResp?.error?.name}
      </Alerts.Danger>
    );
  }

  if (results.some(r => r.status === 'pending')) {
    return <ShimmerBox height="24px" width="100%" />;
  }

  const integration = integrationResp.data;
  const stats = statsResp.data;

  if (!integration || !stats) {
    return (
      <Alerts.Danger>
        There was an error loading this integration.
      </Alerts.Danger>
    );
  }

  return (
    <>
      <AwsOidcHeader integration={integration} />
      <FeatureBox maxWidth={1440} margin="auto" gap={3}>
        <AwsRaTitle integration={integration} />
        <Flex>
          {integration && stats && <ConsoleCardEnrolled stats={stats} />}
        </Flex>
      </FeatureBox>
    </>
  );
}

function AwsRaTitle({ integration }: { integration: IntegrationAwsRa }) {
  const theme = useTheme();
  const history = useHistory();
  const integrationOps = useIntegrationOperation();
  const { status, labelKind } = getStatusAndLabel(integration);

  const anchorIdReg = new RegExp('[^\\/]+$');
  const trustAnchorId = integration?.spec?.trustAnchorARN?.match(anchorIdReg);

  async function removeIntegration() {
    await integrationOps.remove();
    integrationOps.clear();
    history.push(cfg.routes.integrations);
  }

  async function editIntegration(req: EditableIntegrationFields) {
    await integrationOps.edit(req);
    integrationOps.clear();
  }

  return (
    <Flex mt={3} justifyContent="space-between" alignItems="center" mb={4}>
      <Flex alignItems="center" data-testid="aws-oidc-title">
        <HoverTooltip placement="bottom" tipContent="Back to integrations">
          <ButtonIcon
            as={InternalLink}
            to={cfg.routes.integrations}
            aria-label="back"
          >
            <ArrowLeft size="medium" />
          </ButtonIcon>
        </HoverTooltip>
        <Flex flexDirection="column" ml={1}>
          <Flex alignItems="center" gap={2}>
            <Text bold fontSize={6}>
              {integration.name}
            </Text>
            <Label kind={labelKind} aria-label="status" px={3}>
              {status}
            </Label>
          </Flex>
          <Flex gap={1}>
            Trust Anchor ARN:{' '}
            {trustAnchorId && trustAnchorId.length === 1 ? (
              <Link
                target="_blank"
                href={`https://console.aws.amazon.com/rolesanywhere/home/trust-anchors/${trustAnchorId[0]}`}
              >
                <Text
                  style={{
                    fontFamily: theme.fonts.mono,
                  }}
                >
                  {integration.spec?.trustAnchorARN}
                </Text>
              </Link>
            ) : (
              <Text
                style={{
                  fontFamily: theme.fonts.mono,
                }}
              >
                {integration.spec?.trustAnchorARN}
              </Text>
            )}
          </Flex>
        </Flex>
      </Flex>
      <Flex gap={1} alignItems="center">
        <MenuButton icon={<Icons.Cog size="small" />}>
          <MenuItem
            onClick={() =>
              history.push(
                cfg.getIntegrationEnrollRoute(IntegrationKind.AwsRa, 'access'),
                {
                  integrationName: integration.name,
                  trustAnchorArn: integration.spec.trustAnchorARN,
                  syncProfileArn: integration.spec.profileSyncConfig.profileArn,
                  syncRoleArn: integration.spec.profileSyncConfig.roleArn,
                  edit: true,
                }
              )
            }
          >
            Edit…
          </MenuItem>
          <MenuItem onClick={() => integrationOps.onRemove(integration)}>
            Delete…
          </MenuItem>
        </MenuButton>
        <InfoGuideButton config={{ guide: <Guide /> }} />
      </Flex>
      <IntegrationOperations
        operation={integrationOps.type}
        integration={integrationOps.item}
        close={integrationOps.clear}
        edit={editIntegration}
        remove={removeIntegration}
      />
    </Flex>
  );
}
