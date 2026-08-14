import type { JSX } from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import { H2, Subtitle2 } from 'design/Text';
import { AuthType } from 'shared/services';

export function AddNewConnectorTile({
  name,
  kind,
  customDesc,
  isGuided,
  Icon,
  onClick,
}: {
  name: string;
  kind: AuthType;
  customDesc?: string;
  isGuided?: boolean;
  Icon: () => JSX.Element;
  onClick: () => void;
}) {
  let desc: string;
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
    default:
      kind satisfies never | 'local';
  }

  return (
    <AddNewConnectorBox onClick={onClick} tabIndex={0}>
      <Flex
        justifyContent="space-between"
        alignItems="center"
        height="100%"
        gap={3}
      >
        <Icon />
        <Flex flexDirection="column" alignItems="flex-start" gap={1}>
          <Flex alignItems="center" gap={2}>
            <H2>{name}</H2>
          </Flex>
          <Subtitle2
            css={`
              display: -webkit-box;
              -webkit-box-orient: vertical;
              -webkit-line-clamp: 1;
            `}
            color="text.slightlyMuted"
          >
            {customDesc || desc}
          </Subtitle2>
        </Flex>
      </Flex>
      {isGuided && <GuidedBadge>Guided</GuidedBadge>}
    </AddNewConnectorBox>
  );
}

export const AddNewConnectorBox = styled(Box)`
  cursor: pointer;
  height: 96px;
  position: relative;
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[3]}px;
  padding: ${p => p.theme.space[3]}px;
  transition: all 0.3s;

  border-radius: ${props => props.theme.radii[3]}px;
  border: ${props => props.theme.borders[2]}
    ${props => props.theme.colors.interactive.tonal.neutral[0]};

  &:hover {
    background: ${props => props.theme.colors.levels.surface};
    border: ${props => props.theme.borders[2]} transparent;
  }

  &:focus-visible {
    outline: none;
    border: ${props => props.theme.borders[2]}
      ${props => props.theme.colors.text.muted};
  }

  &:active {
    outline: none;
    background: ${props => props.theme.colors.levels.surface};
    border: ${props => props.theme.borders[2]}
      ${props => props.theme.colors.interactive.tonal.neutral[1]};
  }
`;

const GuidedBadge = styled.div`
  position: absolute;
  background: ${props => props.theme.colors.brand};
  color: ${props => props.theme.colors.text.primaryInverse};
  padding: 0px 6px;
  border-top-right-radius: 8px;
  border-bottom-left-radius: 8px;
  top: -2px;
  right: -2px;
  font-size: 10px;
  line-height: 24px;
`;
