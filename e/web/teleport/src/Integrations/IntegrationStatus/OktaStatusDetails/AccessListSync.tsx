import { P3, Flex, Label, Text } from 'design';
import { Link as InternalLink } from 'react-router-dom';
import { SyncAlt } from 'design/Icon';
import { IconTooltip, HoverTooltip } from 'design/Tooltip';

import { OktaAccessListSyncDetails } from 'teleport/services/integrations/oktaStatusTypes';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { CtaEvent } from 'teleport/services/userEvent';

import cfg from 'e-teleport/config';

import {
  Panel,
  PanelTitle,
  CenteredFlex,
  CustomLabel,
  ErrorTooltip,
  TextWithBorderBottom,
  LinkedInnerCard,
} from './Shared';
import { getDurationText } from './date';

export function AccessListsSync({
  spec,
  defaultOwners = [],
}: {
  spec: OktaAccessListSyncDetails;
  defaultOwners: string[];
}) {
  const accessListFeature = cfg.oss.entitlements.AccessLists;
  const hasAccessList =
    accessListFeature.enabled && accessListFeature.limit === 0;

  return (
    <Panel>
      <CenteredFlex>
        <CenteredFlex>
          <PanelTitle>Access Lists Sync</PanelTitle>
          {hasAccessList && (
            <IconTooltip>
              Access Lists enable simple permissions management and provide
              auditing capabilities.
            </IconTooltip>
          )}
        </CenteredFlex>
        <CustomLabel enabled={spec.enabled} />
      </CenteredFlex>
      {!hasAccessList && (
        <>
          <Text color="text.slightlyMuted">
            Synchronization of Okta user groups and Okta applications (with
            direct assignments) imported into Teleport Access Lists. Access
            Lists enable simple permissions management and provide auditing
            capabilities.
          </Text>
          <ButtonLockedFeature mt={3} event={CtaEvent.CTA_ACCESS_LIST}>
            Unlock with Teleport Identity
          </ButtonLockedFeature>
        </>
      )}
      {hasAccessList && (
        <>
          <Flex gap={3}>
            <HoverTooltip tipContent="Go to Access Lists">
              <LinkedInnerCard
                as={InternalLink}
                to={`${cfg.getAccessListManagementRoute()}?search=okta`}
              >
                <Text bold fontSize={6} mb={2}>
                  {spec.numGroups + spec.numApps}
                </Text>
                <Text bold>Access Lists</Text>
                <TextWithBorderBottom color="text.slightlyMuted">
                  Synchronization of Okta user groups and Okta applications
                  (with direct assignments) and each imported as a Teleport
                  Access List.
                </TextWithBorderBottom>
                {defaultOwners.length ? (
                  <Flex mt={2} gap={1} flexWrap="wrap">
                    <Text fontSize={1}>Default Owners:</Text>
                    {defaultOwners.map((label, index) => (
                      <Label key={`${label}${index}`} kind="secondary">
                        {label}
                      </Label>
                    ))}
                  </Flex>
                ) : (
                  <Text color={'text.slightlyMuted'} mt={2}>
                    <i>No default owners defined</i>
                  </Text>
                )}
                {spec.groupFilters.length ? (
                  <Flex mt={2} gap={1} flexWrap="wrap">
                    <Text fontSize={1}>User Groups Filtered By:</Text>
                    {spec.groupFilters.map((label, index) => (
                      <Label key={`${label}${index}`} kind="secondary">
                        {label}
                      </Label>
                    ))}
                  </Flex>
                ) : (
                  <Text color={'text.slightlyMuted'} mt={2}>
                    <i>No user group filters defined</i>
                  </Text>
                )}
                {spec.appFilters.length ? (
                  <Flex mt={2} gap={1} flexWrap="wrap">
                    <Text fontSize={1}>Applications Filtered By:</Text>
                    {spec.appFilters.map((label, index) => (
                      <Label key={`${label}${index}`} kind="secondary">
                        {label}
                      </Label>
                    ))}
                  </Flex>
                ) : (
                  <Text color={'text.slightlyMuted'} mt={2}>
                    <i>No Application filters defined</i>
                  </Text>
                )}
              </LinkedInnerCard>
            </HoverTooltip>
          </Flex>
          <Flex alignItems="center" mt={3}>
            <SyncAlt color="text.slightlyMuted" size="small" mr={1} />
            <P3 color="text.slightlyMuted">
              Last Synced: {getDurationText(spec.lastSuccess)}{' '}
              <ErrorTooltip
                statusCode={spec.statusCode}
                lastFailed={spec.lastFailed}
                error={spec.error}
              />
            </P3>
          </Flex>
        </>
      )}
    </Panel>
  );
}
