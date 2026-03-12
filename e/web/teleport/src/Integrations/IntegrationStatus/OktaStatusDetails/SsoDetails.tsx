import { Fragment } from 'react';
import { useNavigate } from 'react-router';

import { Link as ExternalLink, Flex, Mark, Text } from 'design';
import { NewTab, PlugsConnected } from 'design/Icon';

import { StatusAndOptions } from 'e-teleport/Integrations/IntegrationStatus/Shared';
import useTeleportE from 'e-teleport/useTeleportE';
import cfg from 'teleport/config';
import {
  DefaultSystemOktaRequesterRoleName,
  OktaSsoDetails,
} from 'teleport/services/integrations/oktaStatusTypes';

import { generateOktaSamlAppUrl } from './generateOktaAdminLink';
import { Panel, PanelTitle } from './Shared';

export const SsoDetails = ({
  spec,
  orgUrl,
}: {
  spec?: Pick<
    OktaSsoDetails,
    'appId' | 'appName' | 'enabled' | 'oktaGroupEveryoneMappedRoles'
  >;
  orgUrl?: string;
}) => {
  const ctx = useTeleportE();
  const navigate = useNavigate();
  const hasSsoAccess = ctx.storeUser.getConnectorAccess().list;

  const options = [];
  if (hasSsoAccess) {
    options.push({
      label: 'View Auth Connector',
      onClick: () => navigate(cfg.routes.sso),
      Icon: PlugsConnected,
    });
  }
  if (orgUrl && spec?.appId && spec?.appName) {
    options.push({
      label: "Open Okta's SAML App",
      onClick: () => {
        window.open(
          generateOktaSamlAppUrl({
            orgUrl,
            appId: spec.appId,
            appName: spec.appName,
          }),
          '_blank'
        );
      },
      Icon: NewTab,
    });
  }

  return (
    <Panel>
      <Flex alignItems="center" justifyContent="space-between" gap={2}>
        <Flex alignItems="center" justifyContent="space-between" gap={2}>
          <PanelTitle>SSO Connector</PanelTitle>
        </Flex>
        <StatusAndOptions enabled={spec?.enabled} options={options} />
      </Flex>
      <Flex flexDirection="column" gap={3} px={1} pt={1} height="100%">
        <Text color="text.slightlyMuted">
          <span>
            Your SAML SSO connector that grants Okta users from{' '}
            <ExternalLink
              target="_blank"
              href={orgUrl}
              css={`
                display: inline;
              `}
            >
              {orgUrl}
            </ExternalLink>{' '}
            {formatMappedRoles(spec?.oktaGroupEveryoneMappedRoles ?? [])}
          </span>
        </Text>
        <Flex flexDirection="column" gap={1}>
          {spec?.appId && (
            <Text color="text.slightlyMuted">
              SSO App ID: <b>{spec.appId}</b>
            </Text>
          )}
          {spec?.appName && (
            <Text color="text.slightlyMuted">
              SSO App Name: <b>{spec.appName}</b>
            </Text>
          )}
        </Flex>
      </Flex>
    </Panel>
  );
};

const formatMappedRoles = (roles: string[]) => {
  const isDefaultSystemRequesterRole =
    roles.length === 1 && roles[0] === DefaultSystemOktaRequesterRoleName;

  return (
    <>
      {`the ${isDefaultSystemRequesterRole ? 'default' : ''} Teleport role${roles.length > 1 ? 's' : ''} of`}{' '}
      {roles.map((role, idx) => (
        <Fragment key={role}>
          <Mark>{role}</Mark>
          {idx < roles.length - 1 && ', '}
          {idx === roles.length - 2 && 'and '}
        </Fragment>
      ))}
      .
    </>
  );
};
