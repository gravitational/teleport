import React from 'react';
import { useHistory } from 'react-router';
import styled from 'styled-components';
import { Flex, Box, Text } from 'design';
import { User } from 'design/Icon';

import cfg from 'e-teleport/config';

import { ToolTipText, TruncatingLabel } from '../Shared/Shared';

import { AccessListWithModifiedGrants } from './AccessLists';

export type Props = {
  accessList: AccessListWithModifiedGrants;
};

// TODO(lisa): design is very similar to unifiedresources/ResourceCard.tsx
// consider moving shared styles to a more general place eg: `SingleLineBox`
// and `TruncatingLabel`
export function AccessCard({ accessList }: Props) {
  const { id, title, description, membersCount, grants } = accessList;
  const history = useHistory();

  function handleOnClick() {
    history.push(cfg.getAccessListManagementRoute(id));
  }

  let truncatedDesc = description;
  // Roughly two lines worth of text.
  // TODO(lisa): consider using fixed font size and line height attributes,
  // use a container that fits precisely two lines of text, and then use CSS
  // truncation. This way, you'll get a much more stable and reliable layout.
  if (description.length > 110) {
    truncatedDesc = `${description.substring(0, 110)}...`;
  }

  const canViewMembers = membersCount != null && membersCount > 0;

  return (
    <AccessCardContainer key={id} onClick={handleOnClick}>
      <Box width="100%">
        <SingleLineBox bold title={title}>
          {title}
        </SingleLineBox>
        <Description color="text.muted" title={description}>
          {truncatedDesc}
        </Description>
      </Box>
      <Flex>
        {canViewMembers && (
          <Flex
            alignItems="center"
            title={`${membersCount} members in this list`}
            mr={2}
          >
            <User size={16} />
            <Text ml={1}>{membersCount}</Text>
          </Flex>
        )}
        <Flex>
          {renderRolesAndTraits({
            roles: grants.roles,
            traits: grants.traitList,
          })}
        </Flex>
      </Flex>
    </AccessCardContainer>
  );
}

const renderRolesAndTraits = ({
  roles,
  traits,
}: {
  roles: string[];
  traits: string[];
}) => {
  const combinedRolesAndGrants = [...roles, ...traits];
  const $labels = combinedRolesAndGrants.map((label, index) => (
    <TruncatingLabel
      mr={index === combinedRolesAndGrants.length - 1 ? 0 : 1}
      key={`${label}${index}`}
      kind="secondary"
      title={label}
    >
      {label}
    </TruncatingLabel>
  ));

  // Render at least 2 labels and two lines of label.
  if ($labels.length > 2) {
    const truncatedLabels = $labels.slice(0, 2);
    const otherLabels = $labels.slice(2);
    return (
      <Flex flexWrap="wrap" alignItems="flex-end">
        {truncatedLabels}
        <ToolTipText tipContent={<>{otherLabels}</>}>
          <Text fontSize={0} color="text.muted">
            +{otherLabels.length} more
          </Text>
        </ToolTipText>
      </Flex>
    );
  }

  return <Flex flexWrap="wrap">{$labels}</Flex>;
};

const AccessCardContainer = styled(Flex)`
  transition: all 150ms;

  border-radius: ${props => props.theme.radii[2]}px;
  border: 2px solid ${props => props.theme.colors.spotBackground[0]};

  min-width: 333px;
  max-width: 333px;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  flex-direction: column;
  justify-content: space-between;
  align-items: flex-start;
  gap: 4px;
  flex: 1 1 0;

  &:hover {
    cursor: pointer;
    box-shadow: ${props => props.theme.boxShadow[1]};
    border-color: ${props => props.theme.colors.levels.elevated};
    background-color: ${props => props.theme.colors.levels.elevated};
  }
`;

const Description = styled(Text)`
  font-size: ${props => props.theme.fontSizes[0]}px;
  line-height: 16px;
`;

const SingleLineBox = styled(Text)`
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  width: 100%;
`;
