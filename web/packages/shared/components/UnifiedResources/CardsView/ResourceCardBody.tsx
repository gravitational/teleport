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

import styled from 'styled-components';

import { Box, ButtonLink, Flex, Text } from 'design';
import { makeLabelTag } from 'design/formatters';
import { LabelKind } from 'design/Label';
import { LabelButtonWithIcon } from 'design/Label/LabelButtonWithIcon';
import { HoverTooltip } from 'design/Tooltip';

import { CopyButton } from '../../CopyButton/CopyButton';
import { ResourceActionButtonWrapper } from '../shared/ResourceActionButton';
import { ResourceSelectedIcon } from '../shared/ResourceSelectedIcon';
import { SingleLineBox } from '../shared/SingleLineBox';
import { ResourceCardBodyProps } from './types';

// Since we do a lot of manual resizing and some absolute positioning, we have
// to put some layout constants in place here.
const labelHeight = 20; // px
const labelVerticalMargin = 1; // px
export const labelRowHeight = labelHeight + labelVerticalMargin * 2;

export function ResourceCardBody({
  viewItem,
  visibleInputFields,
  hovered,
  showResourceSelectedIcon,
  showAllLabels,
  labelsInnerContainer,
  shouldDisplayStatusWarning,
  showMoreLabelsButton,
  onMoreLabelsClick,
  numMoreLabels,
  resourceLabelConfig,
  onLabelClick,
}: ResourceCardBodyProps) {
  const { name, requiresRequest, ActionButton, labels, SecondaryIcon } =
    viewItem;
  const { primaryDesc, secondaryDesc } = viewItem.cardViewProps;

  return (
    // MinWidth is important to prevent descriptions from overflowing.
    <Flex flexDirection="column" flex="1" minWidth="0" ml={3} gap={1}>
      <Flex flexDirection="row" alignItems="center" gap={1}>
        <SingleLineBox flex="1">
          <HoverTooltip tipContent={name} showOnlyOnOverflow>
            <Text typography="body1">{name}</Text>
          </HoverTooltip>
        </SingleLineBox>
        {visibleInputFields.copy && hovered && <CopyButton value={name} />}
        <ResourceActionButtonWrapper requiresRequest={requiresRequest}>
          {ActionButton}
        </ResourceActionButtonWrapper>
        {(typeof showResourceSelectedIcon === 'function'
          ? showResourceSelectedIcon(labels)
          : showResourceSelectedIcon) && <ResourceSelectedIcon />}
      </Flex>
      <Flex flexDirection="row" alignItems="center">
        <ResTypeIconBox>
          <SecondaryIcon size={18} />
        </ResTypeIconBox>
        {primaryDesc && (
          <SingleLineBox ml={1} title={primaryDesc}>
            <Text typography="body3" color="text.slightlyMuted">
              {primaryDesc}
            </Text>
          </SingleLineBox>
        )}
        {secondaryDesc && (
          <SingleLineBox ml={2} title={secondaryDesc}>
            <Text typography="body3" color="text.muted">
              {secondaryDesc}
            </Text>
          </SingleLineBox>
        )}
      </Flex>
      <LabelsContainer showAll={showAllLabels}>
        <LabelsInnerContainer
          ref={labelsInnerContainer}
          hasUnhealthyStatus={shouldDisplayStatusWarning}
        >
          <MoreLabelsButton
            style={{
              visibility:
                showMoreLabelsButton && !showAllLabels ? 'visible' : 'hidden',
            }}
            onClick={onMoreLabelsClick}
          >
            + {numMoreLabels} more
          </MoreLabelsButton>
          {labels.map((label, i) => {
            let kind: LabelKind = 'secondary';
            if (resourceLabelConfig?.processLabel) {
              kind = resourceLabelConfig.processLabel(label).kind;
            }

            const labelText = makeLabelTag(label);
            return (
              <StyledLabel
                key={i}
                title={labelText}
                onClick={() => onLabelClick?.(label)}
                withHoverState={!!onLabelClick}
                kind={kind}
                data-is-label=""
                {...resourceLabelConfig}
              >
                {labelText}
              </StyledLabel>
            );
          })}
        </LabelsInnerContainer>
      </LabelsContainer>
    </Flex>
  );
}

/**
 * This box serves twofold purpose: first, it prevents the underlying icon from
 * being squeezed if the parent flexbox starts shrinking items. Second, it
 * prevents the icon from magically occupying too much space, since the SVG
 * element somehow forces the parent to occupy at least full line height.
 */
const ResTypeIconBox = styled(Box)`
  line-height: 0;
`;

/**
 * The outer labels container is resized depending on whether we want to show a
 * single row, or all labels. It hides the internal container's overflow if more
 * than one row of labels exist, but is not yet visible.
 */
const LabelsContainer = styled(Box)<{ showAll?: boolean }>`
  height: ${props => (props.showAll ? 'auto' : `${labelRowHeight}px`)};
  overflow: hidden;
`;

const StyledLabel = styled(LabelButtonWithIcon)`
  height: ${labelHeight}px;
  margin: ${labelVerticalMargin}px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  line-height: ${labelHeight - labelVerticalMargin}px;
`;

/**
 * The inner labels container always adapts to the size of labels.  Its height
 * is measured by the resize observer.
 */
const LabelsInnerContainer = styled(Flex)<{ hasUnhealthyStatus: boolean }>`
  position: relative;
  flex-wrap: wrap;
  align-items: start;
  gap: ${props => props.theme.space[1]}px;
  // Padding is required to prevent the more label button to not collide
  // with the rendered labels. Just a tiny bit more padding needed to
  // accomodate contents getting pushed more to the right when a
  // WarningRightEdgeBadgeIcon renders.
  padding-right: ${p => (p.hasUnhealthyStatus ? '75px' : '74px')};
`;

/**
 * It's important for this button to use absolute positioning; otherwise, its
 * presence in the layout may itself influence the resize logic, potentially
 * causing a feedback loop.
 */
const MoreLabelsButton = styled(ButtonLink)`
  position: absolute;
  right: 0;

  height: ${labelHeight}px;
  margin: ${labelVerticalMargin}px 0;
  min-height: 0;

  background-color: transparent;
  color: ${props => props.theme.colors.text.slightlyMuted};
  font-style: italic;

  transition: visibility 0s;
  transition: background 150ms;

  &:hover {
    background-color: transparent;
  }
`;
