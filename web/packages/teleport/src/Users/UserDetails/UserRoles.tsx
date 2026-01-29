/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { useState } from 'react';

import { Button, Flex, Label, Text } from 'design';
import * as Icons from 'design/Icon';

import {
  ClickableLabel,
  ExpandableContainer,
  SectionParagraph,
  SectionTitle,
  type UserDetailsSectionProps,
} from './UserDetails';

export function UserRoles({ user, onEdit }: UserDetailsSectionProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  const roles = user.roles || [];
  const initialItemCount = 7;
  const rolesToShow = isExpanded ? roles : roles.slice(0, initialItemCount);
  const hasMoreRoles = roles.length > initialItemCount;

  return (
    <>
      <SectionTitle>
        <Flex justifyContent="space-between">
          <span>Roles ({roles.length})</span>
          {onEdit && (
            <Button
              fill="minimal"
              intent="neutral"
              onClick={onEdit}
              size="small"
              gap={1}
            >
              <Icons.Edit size="small" />
              Edit
            </Button>
          )}
        </Flex>
      </SectionTitle>
      <SectionParagraph>
        {roles.length === 0 ? (
          <Text color="text.muted">No roles assigned.</Text>
        ) : (
          <>
            <ExpandableContainer isExpanded={isExpanded}>
              <Flex rowGap={1} columnGap={2} flexWrap="wrap">
                {rolesToShow.map(role => (
                  <Label key={role} kind="secondary">
                    <Flex gap={1}>
                      <Icons.UserIdBadge size={16} />
                      {role}
                    </Flex>
                  </Label>
                ))}
                {hasMoreRoles && !isExpanded && (
                  <ClickableLabel
                    kind="secondary"
                    onClick={() => setIsExpanded(!isExpanded)}
                  >
                    + {roles.length - initialItemCount} more
                  </ClickableLabel>
                )}
              </Flex>
            </ExpandableContainer>
            {hasMoreRoles && isExpanded && (
              <Flex mt={2}>
                <ClickableLabel
                  kind="secondary"
                  onClick={() => setIsExpanded(!isExpanded)}
                >
                  Show less
                </ClickableLabel>
              </Flex>
            )}
          </>
        )}
      </SectionParagraph>
    </>
  );
}
