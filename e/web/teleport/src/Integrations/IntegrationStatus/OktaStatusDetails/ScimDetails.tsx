import { ComponentProps } from 'react';
import { useNavigate } from 'react-router';

import { Flex, Text } from 'design';
import { FeatureName } from 'design/constants';
import { Edit, NewTab } from 'design/Icon';

import {
  OktaIntegrationStepType,
  SCIM_CONFIG,
  UpsellBulletList,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { StatusAndOptions } from 'e-teleport/Integrations/IntegrationStatus/Shared';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import cfg from 'teleport/config';
import { OktaSsoDetails } from 'teleport/services/integrations/oktaStatusTypes';
import { CtaEvent } from 'teleport/services/userEvent';

import { generateOktaScimSettingsUrl } from './generateOktaAdminLink';
import { Panel, PanelTitle } from './Shared';

export function ScimDetails({
  spec,
  orgUrl,
  disabled,
  onToggle,
  toggled,
}: {
  spec?: Pick<OktaSsoDetails, 'appId' | 'appName'>;
  orgUrl?: string;
  toggled?: boolean;
  disabled?: boolean;
  onToggle: () => void;
}) {
  const navigate = useNavigate();
  const hasIdentity = cfg.entitlements.Identity.enabled;
  const showContent = toggled && hasIdentity;
  const options: ComponentProps<typeof StatusAndOptions>['options'] = [
    {
      label: 'Edit Configuration',
      onClick: () =>
        navigate(
          cfg.getIntegrationStatusRoute(
            'okta',
            'okta',
            OktaIntegrationStepType.Scim
          )
        ),
      Icon: Edit,
      disabled: disabled || !toggled,
    },
  ];
  if (orgUrl && spec?.appId && spec?.appName) {
    options.push({
      label: 'Open SCIM Settings in Okta',
      onClick: () => {
        window.open(
          generateOktaScimSettingsUrl({
            orgUrl,
            appId: spec.appId,
            appName: spec.appName,
          }),
          '_blank'
        );
      },
      disabled: false,
      Icon: NewTab,
    });
  }

  return (
    <Panel>
      <Flex alignItems="center" justifyContent="space-between" gap={2}>
        <PanelTitle>SCIM</PanelTitle>
        <StatusAndOptions
          enabled={toggled}
          disabled={!hasIdentity}
          setEnabled={onToggle}
          canDisable={false}
          options={options}
        />
      </Flex>
      {showContent ? (
        <Flex flexDirection="column" gap={3} p={1}>
          <Text color="text.slightlyMuted">
            SCIM enables Okta to push user (and other resource) updates to
            Teleport in near real time, without waiting for the Okta integration
            to run a synchronization.
          </Text>
        </Flex>
      ) : (
        <Flex
          flexDirection="column"
          justifyContent="space-between"
          height="100%"
        >
          <UpsellBulletList bullets={SCIM_CONFIG.bullets} />
          {!hasIdentity && (
            <ButtonLockedFeature event={CtaEvent.CTA_OKTA_SCIM} mt={1}>
              Unlock with {FeatureName.IdentityGovernance}
            </ButtonLockedFeature>
          )}
        </Flex>
      )}
    </Panel>
  );
}
