import { FeatureBox, FeatureHeaderTitle } from 'teleport/components/Layout';
import styled from 'styled-components';
import { Alert, Box, Flex, H3, Indicator, Text } from 'design';
import ResourceEditor from 'teleport/components/ResourceEditor';
import useResources from 'teleport/components/useResources';

import DeleteConnectorDialog from 'teleport/AuthConnectors/DeleteConnectorDialog';

import useTeleport from 'teleport/useTeleport';

import {
  DesktopDescription,
  MobileDescription,
  ResponsiveFeatureHeader,
} from 'teleport/AuthConnectors/styles/AuthConnectors.styles';
import CTAConnectors from 'teleport/AuthConnectors/ConnectorList/CTAConnectors';

import { H2, P } from 'design/Text/Text';

import AddMenu from './AddMenu';
import useAuthConnectors, { State } from './useAuthConnectors';
import templates from './templates';

import AddNewConnectorsList from './AddNewConnectorList/AddNewConnectorList';
import ConnectorList from './ConnectorList';

export default function Container() {
  const state = useAuthConnectors();
  return <AuthConnectors {...state} />;
}

export function AuthConnectors(props: State) {
  const { attempt, items, remove, save, showAuthConnectorsCTA } = props;
  const ctx = useTeleport();
  const isEmpty = items.length === 0;
  const resources = useResources(items, templates);

  const title =
    resources.status === 'creating'
      ? 'Creating a new auth connector'
      : 'Editing auth connector';
  const description =
    'Auth connectors allow Teleport to authenticate users via an external identity source such as Okta, Microsoft Entra ID, GitHub, etc. This authentication method is commonly known as single sign-on (SSO).';

  function handleOnRemove() {
    return remove(resources.item);
  }

  function handleOnSave(content: string) {
    const kind = resources.item.kind;
    const name = resources.item.name;
    const isNew = resources.status === 'creating';
    return save(kind, name, content, isNew);
  }

  return (
    <FeatureBox>
      <ResponsiveFeatureHeader>
        <FeatureHeaderTitle>Auth Connectors</FeatureHeaderTitle>
        <MobileDescription>{description}</MobileDescription>
        {(!showAuthConnectorsCTA || !isEmpty) && (
          <ResponsiveAddMenu>
            <AddMenu
              onClick={resources.create}
              isOidcLocked={ctx.lockedFeatures.authConnectors}
              isSamlLocked={ctx.lockedFeatures.authConnectors}
            />
          </ResponsiveAddMenu>
        )}
      </ResponsiveFeatureHeader>
      {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <Flex alignItems="start">
          <Flex flexDirection="column" width="100%" gap={5}>
            <Box>
              <H2 mb={4}>Your Connectors</H2>
              <ConnectorList
                items={items}
                onEdit={resources.edit}
                onDelete={resources.remove}
              />
            </Box>
            {isEmpty && !showAuthConnectorsCTA && (
              <AddNewConnectorsList onCreate={resources.create} />
            )}
            {showAuthConnectorsCTA && <CTAConnectors />}
          </Flex>
          <DesktopDescription>
            <H3 mb={3}>Auth Connectors</H3>
            <P>{description}</P>
            <P>
              Please{' '}
              <Text
                as="a"
                color="text.main"
                href="https://goteleport.com/docs/enterprise/sso/"
                target="_blank"
              >
                view our documentation
              </Text>{' '}
              for samples of each connector.
            </P>
          </DesktopDescription>
        </Flex>
      )}
      {(resources.status === 'creating' || resources.status === 'editing') && (
        <ResourceEditor
          title={title}
          onSave={handleOnSave}
          text={resources.item.content}
          name={resources.item.name}
          isNew={resources.status === 'creating'}
          onClose={resources.disregard}
        />
      )}
      {resources.status === 'removing' && (
        <DeleteConnectorDialog
          name={resources.item.name}
          onClose={resources.disregard}
          onDelete={handleOnRemove}
        />
      )}
    </FeatureBox>
  );
}

const ResponsiveAddMenu = styled(Box)`
  width: 240px;
  margin-left: auto;
  align-self: center;

  @media screen and (max-width: ${props => props.theme.breakpoints.tablet}px) {
    width: 100%;
  }
`;
