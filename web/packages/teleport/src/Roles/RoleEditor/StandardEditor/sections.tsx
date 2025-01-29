/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { useTheme } from 'styled-components';

import Box from 'design/Box';
import ButtonIcon from 'design/ButtonIcon';
import Flex from 'design/Flex';
import { Minus, Plus, Trash } from 'design/Icon';
import { H3 } from 'design/Text';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';
import { useValidation } from 'shared/components/Validation';
import { ValidationResult } from 'shared/components/Validation/rules';

import { StandardModelDispatcher } from './useStandardModel';

/** Properties of a section that uses plain callbacks to change the model. */
export type SectionProps<Model, ValidationResult> = {
  value: Model;
  isProcessing: boolean;
  validation?: ValidationResult;
  onChange(value: Model): void;
};

/** Properties of a section that uses a dispatcher to change the model. */
export type SectionPropsWithDispatch<Model, ValidationResult> = {
  value: Model;
  isProcessing: boolean;
  validation?: ValidationResult;
  dispatch: StandardModelDispatcher;
};

/**
 * A wrapper for editor section. Its responsibility is rendering a header,
 * expanding, collapsing, and removing the section.
 */
export const SectionBox = ({
  title,
  tooltip,
  children,
  removable,
  isProcessing,
  validation,
  onRemove,
}: React.PropsWithChildren<{
  title: string;
  tooltip: string;
  removable?: boolean;
  isProcessing: boolean;
  validation?: ValidationResult;
  onRemove?(): void;
}>) => {
  const theme = useTheme();
  const [expanded, setExpanded] = useState(true);
  const ExpandIcon = expanded ? Minus : Plus;
  const expandTooltip = expanded ? 'Collapse' : 'Expand';
  const validator = useValidation();

  const handleExpand = (e: React.MouseEvent) => {
    // Don't let <summary> handle the event, we'll do it ourselves to keep
    // track of the state.
    e.preventDefault();
    setExpanded(expanded => !expanded);
  };

  const handleRemove = (e: React.MouseEvent) => {
    // Don't let <summary> handle the event.
    e.stopPropagation();
    onRemove?.();
  };

  return (
    <Box
      as="details"
      open={expanded}
      border={1}
      borderColor={
        validator.state.validating && !validation.valid
          ? theme.colors.interactive.solid.danger.default
          : theme.colors.interactive.tonal.neutral[0]
      }
      borderRadius={3}
    >
      <Flex
        as="summary"
        height="56px"
        alignItems="center"
        ml={3}
        mr={3}
        css={'cursor: pointer'}
        onClick={handleExpand}
      >
        {/* TODO(bl-nero): Show validation result in the summary. */}
        <Flex flex="1" gap={2}>
          <H3>{title}</H3>
          {tooltip && <IconTooltip>{tooltip}</IconTooltip>}
        </Flex>
        {removable && (
          <Box
            borderRight={1}
            borderColor={theme.colors.interactive.tonal.neutral[0]}
          >
            <HoverTooltip tipContent="Remove section">
              <ButtonIcon
                aria-label="Remove section"
                disabled={isProcessing}
                onClick={handleRemove}
              >
                <Trash
                  size="small"
                  color={theme.colors.interactive.solid.danger.default}
                />
              </ButtonIcon>
            </HoverTooltip>
          </Box>
        )}
        <HoverTooltip tipContent={expandTooltip}>
          <ExpandIcon size="small" color={theme.colors.text.muted} ml={2} />
        </HoverTooltip>
      </Flex>
      <Box px={3} pb={3}>
        {children}
      </Box>
    </Box>
  );
};
