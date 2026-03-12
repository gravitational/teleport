/**
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

import { useNavigate } from 'react-router';

import { Flex, Text } from 'design';
import { FeatureName } from 'design/constants';
import { Edit } from 'design/Icon';

import {
  IDENTITY_SECURITY_SYNC_CONFIG,
  OktaIntegrationStepType,
  UpsellBulletList,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { StatusAndOptions } from 'e-teleport/Integrations/IntegrationStatus/Shared';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import cfg from 'teleport/config';
import { CtaEvent } from 'teleport/services/userEvent';

import { Panel, PanelTitle } from './Shared';

export function IdentitySecuritySyncDetails({
  accessGraphEnabled,
  disabled,
  syncEnabled,
  onToggle,
}: {
  accessGraphEnabled: boolean;
  disabled?: boolean;
  syncEnabled?: boolean;
  onToggle: () => void;
}) {
  const navigate = useNavigate();
  const showContent = accessGraphEnabled && syncEnabled;

  return (
    <Panel>
      <Flex alignItems="center" justifyContent="space-between" gap={2}>
        <PanelTitle>Identity Security Sync</PanelTitle>
        <StatusAndOptions
          enabled={syncEnabled}
          disabled={!accessGraphEnabled}
          setEnabled={onToggle}
          options={[
            {
              label: 'Edit Configuration',
              onClick: () =>
                navigate(
                  cfg.getIntegrationStatusRoute(
                    'okta',
                    'okta',
                    OktaIntegrationStepType.IdentitySecuritySync
                  )
                ),
              Icon: Edit,
              disabled: disabled || !syncEnabled,
            },
          ]}
        />
      </Flex>
      <Flex flexDirection="column" justifyContent="space-between" height="100%">
        {showContent ? (
          <Flex flexDirection="column" gap={2} px={2} pt={1}>
            <Text color="text.slightlyMuted">
              The Okta audit log is being synced to Teleport Identity.
            </Text>
          </Flex>
        ) : (
          <>
            <UpsellBulletList bullets={IDENTITY_SECURITY_SYNC_CONFIG.bullets} />
            {!accessGraphEnabled && (
              <ButtonLockedFeature
                event={CtaEvent.CTA_IDENTITY_SECURITY}
                mt={1}
              >
                Unlock with {FeatureName.IdentitySecurity}
              </ButtonLockedFeature>
            )}
          </>
        )}
      </Flex>
    </Panel>
  );
}
