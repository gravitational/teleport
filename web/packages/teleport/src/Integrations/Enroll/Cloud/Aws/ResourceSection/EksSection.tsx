/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Box, Flex } from '@gravitational/design-system';
import { useState } from 'react';

import { FieldCheckbox } from 'shared/components/FieldCheckbox';

import { TextWeighted, TextWeightedSmall } from '../../Shared';
import { TagsButton, TagsChevron } from '../Tags';

export function KubeAppDiscoverySection({
  disabled,
  isChecked,
  onUpdate,
}: {
  disabled: boolean;
  isChecked: boolean;
  onUpdate(): void;
}) {
  const [expanded, setExpanded] = useState(true);

  return (
    <Box mt={3} mb={2}>
      <TagsButton onClick={() => setExpanded(prev => !prev)}>
        <Flex alignItems="center" gap={1} mb={2}>
          <TagsChevron size="medium" expanded={expanded} />
          <TextWeighted>
            Advanced -{' '}
            <span css="font-style: italic;">Kubernetes App Discovery</span>
          </TextWeighted>
        </Flex>
      </TagsButton>
      <TextWeightedSmall ml={4} mt={-2}>
        Teleport’s Kubernetes App Discovery will automatically identify and
        enroll HTTP applications running inside discovered Kubernetes clusters
      </TextWeightedSmall>
      {expanded && (
        <Box ml={5} mt={2}>
          <FieldCheckbox
            mb={0}
            size="small"
            label={'Enable Kubernetes App Discovery'}
            labelFontWeight="regular"
            checked={isChecked}
            disabled={disabled}
            onChange={onUpdate}
          />
        </Box>
      )}
    </Box>
  );
}
