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

import styled from 'styled-components';

import { Button, Flex, Label, Text } from 'design';
import * as Icons from 'design/Icon';

import {
  SectionParagraph,
  SectionTitle,
  type UserDetailsSectionProps,
} from './UserDetails';

export function UserTraits({ user, onEdit }: UserDetailsSectionProps) {
  const allTraits = user.allTraits || [];
  const traitsWithValues = Object.keys(allTraits).filter(
    key =>
      user.allTraits[key].length > 0 &&
      user.allTraits[key].some(value => value.trim() !== '')
  );

  return (
    <>
      <SectionTitle>
        <Flex justifyContent="space-between">
          <span>Traits</span>
          {onEdit && (
            <Button
              size="small"
              fill="minimal"
              intent="neutral"
              onClick={onEdit}
              gap={1}
            >
              <Icons.Edit size="small" />
              Edit
            </Button>
          )}
        </Flex>
      </SectionTitle>
      <SectionParagraph>
        {traitsWithValues.length === 0 ? (
          <Text color="text.muted">No traits assigned.</Text>
        ) : (
          <Flex flexWrap="wrap" gap={2}>
            {traitsWithValues.map(key => (
              <StyledLabel key={key} kind="secondary">
                {key}: {user.allTraits[key].join(', ')}
              </StyledLabel>
            ))}
          </Flex>
        )}
      </SectionParagraph>
    </>
  );
}

// border-radius matches label height (10px font-size + 2px margin)
// otherwise, a label with many traits will look like an egg
const StyledLabel = styled(Label)`
  border-radius: 12px;
`;
