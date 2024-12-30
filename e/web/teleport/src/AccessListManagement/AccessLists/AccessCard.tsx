import { cloneElement } from 'react';
import { format } from 'date-fns';
import styled from 'styled-components';
import { Flex, Box, Text } from 'design';
import { User, UserList } from 'design/Icon';

import { pluralize } from 'shared/utils/text';

import { HoverTooltip } from 'design/Tooltip';

import { AccessListType } from 'e-teleport/services/accessmanagement';

import { TruncatingLabel } from '../Shared/Shared';
import { TypeBadge } from '../Shared/TypeBadge';

import { AccessListWithModifiedGrants } from './AccessLists';

export type Props = {
  accessList: AccessListWithModifiedGrants;
  // onlyRender flag makes access card non-interactable.
  onlyRender?: boolean;
  onClick(): void;
};

// TODO(lisa): design is very similar to unifiedresources/ResourceCard.tsx
// consider moving shared styles to a more general place eg: `SingleLineBox`
// and `TruncatingLabel`
export function AccessCard({ accessList, onlyRender = false, onClick }: Props) {
  const {
    id,
    title,
    description,
    membersCount,
    memberListCount,
    grants,
    needsReviewBy,
    type,
  } = accessList;
  let truncatedDesc = description;
  // Roughly two lines worth of text.
  // TODO(lisa): consider using fixed font size and line height attributes,
  // use a container that fits precisely two lines of text, and then use CSS
  // truncation. This way, you'll get a much more stable and reliable layout.
  if (description?.length > 110) {
    truncatedDesc = `${description.substring(0, 110)}...`;
  }

  // Users can have permission levels of Member, Owner, or Admin.
  // If the value of `membersCount` is null, the user does not
  // have permission to list other members, which is only the
  // case for Members.
  const isMember = membersCount == null;
  const canViewMembers = !isMember && membersCount >= 0;
  const requiresReview = needsReviewBy && !isMember;
  const isOverdue = requiresReview && needsReviewBy < new Date();

  return (
    <AccessCardContainer
      key={id}
      onClick={onClick}
      onKeyUp={e => (e.key === 'Enter' || e.key === ' ' ? onClick() : null)}
      $onlyRender={onlyRender}
      tabIndex={0}
      role="listitem"
    >
      {requiresReview && (
        <ReviewBadge isOverdue={isOverdue}>
          Review by {format(needsReviewBy, 'MM/dd')}
        </ReviewBadge>
      )}
      <Box width="100%">
        <Flex gap={1}>
          <SingleLineBox bold title={title} $requiresReview={requiresReview}>
            {title}
          </SingleLineBox>
          {type !== AccessListType.Unspecified && <TypeBadge type={type} />}
        </Flex>
        <Text typography="body4" color="text.muted" title={description}>
          {truncatedDesc}
        </Text>
      </Box>
      <Flex>
        {canViewMembers && (
          <Flex alignItems="center" gap={2} mr={2}>
            <Flex
              alignItems="center"
              title={`${memberListCount} Access ${pluralize(memberListCount, 'List')} in this list`}
            >
              <UserList size={16} />
              <Text ml={1} typography="body4">
                {memberListCount || '0'}
              </Text>
            </Flex>
            <Flex
              alignItems="center"
              title={`${membersCount} ${pluralize(membersCount, 'member')} in this list`}
            >
              <User size={16} />
              <Text ml={1} typography="body4">
                {membersCount}
              </Text>
            </Flex>
          </Flex>
        )}
        <Flex>
          {renderRolesAndTraits({
            roles: grants.roles || [],
            traits: grants.traitList || [],
          })}
        </Flex>
      </Flex>
    </AccessCardContainer>
  );
}

export const renderRolesAndTraits = ({
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
      <Flex flexWrap="wrap" alignItems="baseline">
        {truncatedLabels}
        <HoverTooltip
          position="bottom"
          tipContent={otherLabels.map(label =>
            // Labels in the tip content need to be rendered in inverse colors,
            // or they will be illegible.
            cloneElement(label, { inverse: true })
          )}
        >
          <Text typography="body4">+ {otherLabels.length} more</Text>
        </HoverTooltip>
      </Flex>
    );
  }

  return <Flex flexWrap="wrap">{$labels}</Flex>;
};

const AccessCardContainer = styled(Flex)<{ $onlyRender?: boolean }>`
  position: relative;
  transition:
    background-color 150ms ease,
    border-color 150ms ease,
    outline-width 150ms ease;

  border-radius: ${props => props.theme.radii[2]}px;
  border: 2px solid ${props => props.theme.colors.spotBackground[0]};

  width: 100%;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  flex-direction: column;
  justify-content: space-between;
  align-items: flex-start;
  gap: 4px;
  outline: none;

  &:focus-visible {
    outline: ${p => p.theme.borders[2]} ${props => props.theme.colors.brand};
  }

  &:hover,
  &:focus-visible {
    cursor: pointer;
    box-shadow: ${props => props.theme.boxShadow[1]};
    border-color: ${props => props.theme.colors.levels.elevated};
    background-color: ${props => props.theme.colors.levels.elevated};
  }

  ${p => {
    if (p.$onlyRender) {
      return {
        pointerEvents: 'none',
        cursor: 'pointer',
        boxShadow: p.theme.boxShadow[1],
        borderColor: p.theme.colors.levels.elevated,
        backgroundColor: p.theme.colors.levels.elevated,
      };
    }
  }}
`;

const SingleLineBox = styled(Text)<{ $requiresReview: boolean }>`
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  max-width: ${p => (p.$requiresReview ? '155' : '235')}px;
`;

const ReviewBadge = styled.div<{ isOverdue?: boolean }>`
  position: absolute;
  background-color: ${p =>
    p.isOverdue ? p.theme.colors.error.main : p.theme.colors.warning.main};
  width: 80px;
  height: 20px;
  right: 0;
  border-bottom-left-radius: ${p => p.theme.radii[2]}px;
  border-top-left-radius: ${p => p.theme.radii[2]}px;
  display: flex;
  align-items: center;
  flex-direction: row-reverse;
  padding-right: ${p => p.theme.space[2]}px;
  margin-top: 2px;
  color: ${p => p.theme.colors.dark};

  ${p => p.theme.typography.body4}
`;
