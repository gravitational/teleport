import React from 'react';
import { useTheme } from 'styled-components';

import { Flex, H2, Subtitle2, ButtonSecondary, P3, Label } from 'design';
import {
  ArrowRight,
  CircleCheck,
  MinusCircle,
  Password,
  RocketLaunch,
} from 'design/Icon';

import { MenuIcon, MenuItem } from 'shared/components/MenuAction';
import { AuthType } from 'shared/services';

import { State as ResourceState } from 'teleport/components/useResources';
import { ConnectorBox } from 'teleport/AuthConnectors/styles/ConnectorBox.styles';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { CtaEvent } from 'teleport/services/userEvent';

export function AuthConnectorTile({
  id,
  name,
  kind,
  Icon,
  isDefault,
  isPlaceholder,
  onSetup,
  isEnabled,
  customDesc,
  onEdit,
  onDelete,
  isCTA = false,
}: Props) {
  const theme = useTheme();
  const onClickEdit = () => onEdit(id);
  const onClickDelete = () => onDelete(id);

  let desc;
  switch (kind) {
    case 'github':
      desc = 'GitHub Connector';
      break;
    case 'oidc':
      desc = 'OIDC Connector';
      break;
    case 'saml':
      desc = 'SAML Connector';
      break;
  }

  const ActionsSection = () => {
    if (!isCTA && !isPlaceholder) {
      return <EnabledIndicator isEnabled={isEnabled} />;
    }

    // If this is a placeholder tile, only show a "Set Up" button.
    if (isPlaceholder && !!onSetup) {
      return (
        <ButtonSecondary
          onClick={onSetup}
          px={3}
          css={`
            position: absolute;
          `}
        >
          Set Up <ArrowRight size="small" ml={2} />
        </ButtonSecondary>
      );
    }

    if (isCTA) {
      return (
        <ButtonLockedFeature
          event={CtaEvent.CTA_AUTH_CONNECTOR}
          noIcon
          css={`
            border-radius: 62px;
            font-weight: 400;
            width: 185px;
            height: 32px;
            position: absolute;
          `}
          size="small"
          px={3}
          py={2}
        >
          <RocketLaunch size="small" mr={2} />
          Upgrade to Enterprise
        </ButtonLockedFeature>
      );
    }
  };

  return (
    <ConnectorBox>
      <Flex
        flexDirection="column"
        justifyContent="space-between"
        alignItems="flex-start"
        height="100%"
      >
        <Icon />
        <Flex flexDirection="column" alignItems="flex-start" gap={1}>
          <Flex alignItems="center" gap={2}>
            <H2>{name}</H2>
            {isDefault && (
              <Label
                kind="secondary"
                css={`
                  color: ${props => props.theme.colors.text.slightlyMuted};
                `}
              >
                Default
              </Label>
            )}
          </Flex>
          <Subtitle2 color="text.slightlyMuted">{customDesc || desc}</Subtitle2>
        </Flex>
      </Flex>
      <Flex
        flexDirection="column"
        justifyContent="space-between"
        alignItems="flex-end"
        height="100%"
      >
        <ActionsSection />
        {!!onEdit && !!onDelete && (
          <MenuIcon
            buttonIconProps={{ style: { borderRadius: `${theme.radii[2]}px` } }}
          >
            <MenuItem onClick={onClickEdit}>Edit</MenuItem>
            <MenuItem onClick={onClickDelete}>Delete</MenuItem>
          </MenuIcon>
        )}
      </Flex>
    </ConnectorBox>
  );
}

/**
 * LocalConnectorTile is a hardcoded "auth connector" which represents local auth.
 */
export function LocalConnectorTile() {
  return (
    <AuthConnectorTile
      key="local-auth-tile"
      kind="local"
      id="local"
      Icon={() => (
        <Flex
          alignItems="center"
          justifyContent="center"
          css={`
            background-color: ${props =>
              props.theme.colors.interactive.tonal.neutral[0]};
            height: 61px;
            width: 61px;
          `}
          lineHeight={0}
          p={2}
          borderRadius={3}
        >
          <Password size="extra-large" />
        </Flex>
      )}
      isDefault={true}
      isPlaceholder={false}
      isEnabled={true}
      name="Local Connector"
      customDesc="Manual auth w/ users local to Teleport"
    />
  );
}

function EnabledIndicator({ isEnabled }: { isEnabled: boolean }) {
  return (
    <Flex
      justifyContent="center"
      alignItems="center"
      gap={1}
      py={1}
      px={2}
      css={`
        background-color: ${isEnabled
          ? props => props.theme.colors.interactive.tonal.success[0]
          : props => props.theme.colors.interactive.tonal.neutral[0]};
        width: 85px;
        height: 24px;
        border-radius: 62px;
      `}
    >
      {isEnabled ? (
        <>
          <CircleCheck color="interactive.solid.success.default" size="small" />
          <P3 color="interactive.solid.success.default">Enabled</P3>
        </>
      ) : (
        <>
          <MinusCircle color="text.disabled" size="small" />
          <P3 color="text.disabled">Disabled</P3>
        </>
      )}
    </Flex>
  );
}

type Props = {
  name: string;
  id: string;
  kind: AuthType;
  Icon: () => JSX.Element;
  isDefault: boolean;
  /**
   * isPlaceholder is whether this isn't a real existing connector, but a placeholder as a shortcut to set one up.
   */
  isPlaceholder: boolean;
  onSetup?: () => void;
  customDesc?: string;
  isEnabled: boolean;
  onEdit?: ResourceState['edit'];
  onDelete?: ResourceState['remove'];
  /**
   * isCTA is whether this tile isn't a real existing conncetor, but a CTA (call to action) to upgrade to Enterprise.
   */
  isCTA?: boolean;
};
