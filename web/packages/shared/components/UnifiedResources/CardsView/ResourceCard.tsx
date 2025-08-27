/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { Ref, useEffect, useLayoutEffect, useRef, useState } from 'react';
import styled, { css } from 'styled-components';

import { Box, ButtonLink, Flex, Label, Text } from 'design';
import { CheckboxInput } from 'design/Checkbox';
import { ResourceIcon } from 'design/ResourceIcon';
import { HoverTooltip } from 'design/Tooltip';

// eslint-disable-next-line no-restricted-imports -- FIXME
import { makeLabelTag } from 'teleport/components/formatters';

import { CopyButton } from '../shared/CopyButton';
import {
  BackgroundColorProps,
  getBackgroundColor,
  getStatusBackgroundColor,
} from '../shared/getBackgroundColor';
import { PinButton } from '../shared/PinButton';
import { ResourceActionButtonWrapper } from '../shared/ResourceActionButton';
import { SingleLineBox } from '../shared/SingleLineBox';
import { shouldWarnResourceStatus } from '../shared/StatusInfo';
import { ResourceItemProps } from '../types';
import { WarningRightEdgeBadgeSvg } from './WarningRightEdgeBadgeSvg';

// Since we do a lot of manual resizing and some absolute positioning, we have
// to put some layout constants in place here.
const labelHeight = 20; // px
const labelVerticalMargin = 1; // px
const labelRowHeight = labelHeight + labelVerticalMargin * 2;

/**
 * This box serves twofold purpose: first, it prevents the underlying icon from
 * being squeezed if the parent flexbox starts shrinking items. Second, it
 * prevents the icon from magically occupying too much space, since the SVG
 * element somehow forces the parent to occupy at least full line height.
 */
const ResTypeIconBox = styled(Box)`
  line-height: 0;
`;

export function ResourceCard({
  onLabelClick,
  pinningSupport,
  pinned,
  pinResource,
  selectResource,
  selected,
  onShowStatusInfo,
  showingStatusInfo,
  viewItem,
}: Omit<ResourceItemProps, 'expandAllLabels'>) {
  const {
    name,
    primaryIconName,
    SecondaryIcon,
    cardViewProps,
    ActionButton,
    labels,
    requiresRequest,
    status,
  } = viewItem;
  const { primaryDesc, secondaryDesc } = cardViewProps;

  const [showMoreLabelsButton, setShowMoreLabelsButton] = useState(false);
  const [showAllLabels, setShowAllLabels] = useState(false);
  const [numMoreLabels, setNumMoreLabels] = useState(0);

  const [hovered, setHovered] = useState(false);

  const innerContainer = useRef<HTMLDivElement | null>(null);
  const labelsInnerContainer = useRef<HTMLDivElement>(null);
  const collapseTimeout = useRef<ReturnType<typeof setTimeout>>(null);

  // This effect installs a resize observer whose purpose is to detect the size
  // of the component that contains all the labels. If this component is taller
  // than the height of a single label row, we show a "+x more" button.
  useLayoutEffect(() => {
    if (!labelsInnerContainer.current) return;

    // TODO(ravicious): Use useResizeObserver instead. Ensure that the callback passed to
    // useResizeObserver has a stable identity.
    const observer = new ResizeObserver(entries => {
      const container = entries[0];

      // In Connect, when a tab becomes active, its outermost DOM element switches from `display:
      // none` to `display: flex`. This callback is then fired with the height reported as zero.
      //
      // As such, when checking whether to show the "More labels" button, we should consider only
      // values other than zero.
      if (container.contentRect.height === 0) {
        return;
      }

      // We're taking labelRowHeight * 1.5 just in case some glitch adds or
      // removes a pixel here and there.
      const moreThanOneRow =
        container.contentBoxSize[0].blockSize > labelRowHeight * 1.5;
      setShowMoreLabelsButton(moreThanOneRow);

      // Count number of labels in the first row. This will let us calculate and
      // show the number of labels left out from the view.
      const labelElements = [
        ...entries[0].target.querySelectorAll('[data-is-label]'),
      ];
      const firstLabelPos = labelElements[0]?.getBoundingClientRect().top;
      // Find the first one below.
      const firstLabelInSecondRow = labelElements.findIndex(
        e => e.getBoundingClientRect().top > firstLabelPos
      );

      setNumMoreLabels(
        firstLabelInSecondRow > 0
          ? labelElements.length - firstLabelInSecondRow
          : 0
      );
    });

    observer.observe(labelsInnerContainer.current);
    return () => {
      observer.disconnect();
    };
  }, []);

  // Clear the timeout on unmount to prevent changing a state of an unmounted
  // component.
  useEffect(() => () => clearTimeout(collapseTimeout.current), []);

  const onMoreLabelsClick = () => {
    setShowAllLabels(true);
  };

  const onMouseLeave = () => {
    // If the user expanded the labels and then scrolled down enough to hide the
    // top of the card, we scroll back up and collapse the labels with a small
    // delay to keep the user from losing focus on the card that they were
    // looking at. The delay is picked by hand, since there's no (easy) way to
    // know when the animation ends.
    if (
      showAllLabels &&
      (innerContainer.current?.getBoundingClientRect().top ?? 0) < 0
    ) {
      innerContainer.current?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
      });
      clearTimeout(collapseTimeout.current);
      collapseTimeout.current = setTimeout(() => setShowAllLabels(false), 700);
    } else {
      // Otherwise, we just collapse the labels immediately.
      setShowAllLabels(false);
    }
  };

  const shouldDisplayStatusWarning = shouldWarnResourceStatus(status);

  return (
    <CardContainer
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      showingStatusInfo={showingStatusInfo}
    >
      <CardOuterContainer
        showAllLabels={showAllLabels}
        shouldDisplayWarning={shouldDisplayStatusWarning}
      >
        <CardInnerContainer
          ref={innerContainer}
          p={3}
          // we set padding left a bit larger so we can have space to absolutely
          // position the pin/checkbox buttons
          pl={6}
          alignItems="start"
          onMouseLeave={onMouseLeave}
          pinned={pinned}
          requiresRequest={requiresRequest}
          selected={selected}
          showingStatusInfo={showingStatusInfo}
          shouldDisplayWarning={shouldDisplayStatusWarning}
          // we set extra padding to push contents to the right to make
          // space for the WarningRightEdgeBadgeIcon.
          {...(shouldDisplayStatusWarning && !showAllLabels && { pr: '35px' })}
          {...(shouldDisplayStatusWarning && showAllLabels && { pr: '7px' })}
        >
          <CheckboxInput
            checked={selected}
            onChange={selectResource}
            style={{ position: 'absolute', top: '16px', left: '16px' }}
          />
          <Box
            css={`
              position: absolute;
              // we position far from the top so the layout of the pin doesn't change if we expand the card
              top: ${props => props.theme.space[9]}px;
              transition: none;
              left: 16px;
            `}
          >
            <PinButton
              setPinned={pinResource}
              pinned={pinned}
              pinningSupport={pinningSupport}
              hovered={hovered}
            />
          </Box>
          <ResourceIcon
            name={primaryIconName}
            width="45px"
            height="45px"
            ml={2}
            css={`
              opacity: ${requiresRequest ? '0.5' : '1'};
            `}
          />
          {/* MinWidth is important to prevent descriptions from overflowing. */}
          <Flex flexDirection="column" flex="1" minWidth="0" ml={3} gap={1}>
            <Flex flexDirection="row" alignItems="center">
              <SingleLineBox flex="1">
                <HoverTooltip tipContent={name} showOnlyOnOverflow>
                  <Text typography="body1">{name}</Text>
                </HoverTooltip>
              </SingleLineBox>
              {hovered && <CopyButton name={name} mr={2} />}
              <ResourceActionButtonWrapper requiresRequest={requiresRequest}>
                {ActionButton}
              </ResourceActionButtonWrapper>
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
                      showMoreLabelsButton && !showAllLabels
                        ? 'visible'
                        : 'hidden',
                  }}
                  onClick={onMoreLabelsClick}
                >
                  + {numMoreLabels} more
                </MoreLabelsButton>
                {labels.map((label, i) => {
                  const labelText = makeLabelTag(label);
                  return (
                    <StyledLabel
                      key={i}
                      title={labelText}
                      onClick={() => onLabelClick?.(label)}
                      kind="secondary"
                      data-is-label=""
                    >
                      {labelText}
                    </StyledLabel>
                  );
                })}
              </LabelsInnerContainer>
            </LabelsContainer>
          </Flex>
          {shouldDisplayStatusWarning && !showAllLabels && (
            <HoverTooltip tipContent={'Show Connection Issue'} placement="left">
              <WarningRightEdgeBadgeIcon onClick={onShowStatusInfo} />
            </HoverTooltip>
          )}
        </CardInnerContainer>
      </CardOuterContainer>
      {/* This is to let the WarningRightEdgeBadgeIcon stay in place while the
        InnerContainer pops out and expands vertically from rendering all
        labels. */}
      {shouldDisplayStatusWarning && showAllLabels && (
        <WarningRightEdgeBadgeIcon />
      )}
    </CardContainer>
  );
}

const WarningRightEdgeBadgeIcon = ({
  onClick,
  ref,
}: {
  onClick?(): void;
  ref?: Ref<HTMLDivElement>;
}) => {
  return (
    <Box
      onClick={onClick}
      ref={ref}
      css={`
        position: absolute;
        top: 0;
        right: 0;
        cursor: pointer;
        height: 100%;
      `}
    >
      <WarningRightEdgeBadgeSvg />
    </Box>
  );
};

/**
 * The outer container's purpose is to reserve horizontal space on the resource
 * grid. It holds the inner container that normally holds a regular layout of
 * the card, and is fully contained inside the outer container.  Once the user
 * clicks the "more" button, the inner container "pops out" by changing its
 * position to absolute.
 *
 * The card height is fixed to allow the WarningRightEdgeBadgeIcon to stay in
 * place when user clicks on "showAllLabels". Without the fixed height, the
 * container's height shrinks when the inner container pops out, resulting in
 * the svg to jump around (from size difference) and or disappearing.
 *
 * TODO(bl-nero): Known issue: this doesn't really work well with one-column
 * layout;
 */
const CardContainer = styled(Box)<{
  showingStatusInfo: boolean;
}>`
  height: 110px;

  position: relative;
  .resource-health-status-svg {
    width: 100%;
    height: 100%;

    fill: ${p =>
      p.showingStatusInfo
        ? p.theme.colors.interactive.solid.alert.active
        : p.theme.colors.interactive.solid.alert.default};
  }
  &:hover {
    .resource-health-status-svg {
      fill: ${p => p.theme.colors.interactive.solid.alert.hover};
    }
  }
`;

const CardOuterContainer = styled(Box)<{
  showAllLabels?: boolean;
  shouldDisplayWarning: boolean;
}>`
  border-radius: ${props => props.theme.radii[3]}px;

  ${props =>
    props.showAllLabels &&
    css`
      position: absolute;
      left: 0;
      // The padding is required to show the WarningRightEdgeBadgeIcon
      right: ${props.shouldDisplayWarning ? '28px' : 0};
      z-index: 1;
    `}
  transition: all 150ms;

  // Using double ampersand because of https://github.com/styled-components/styled-components/issues/3678.
  ${CardContainer}:hover && {
    background-color: ${props => props.theme.colors.levels.surface};

    // We use a pseudo element for the shadow with position: absolute in order to prevent
    // the shadow from increasing the size of the layout and causing scrollbar flicker.
    &:after {
      box-shadow: ${props => props.theme.boxShadow[3]};
      border-radius: ${props => props.theme.radii[3]}px;
      content: '';
      position: absolute;
      top: 0;
      left: 0;
      z-index: -1;
      width: 100%;
      height: 100%;
    }
  }
`;

/**
 * The inner container that normally holds a regular layout of the card, and is
 * fully contained inside the outer container.  Once the user clicks the "more"
 * button, the inner container "pops out" by changing its position to absolute.
 *
 * TODO(bl-nero): Known issue: this doesn't really work well with one-column
 * layout; we may need to globally set the card height to fixed size on the
 * outer container.
 */
const CardInnerContainer = styled(Flex)<BackgroundColorProps>`
  border: ${props => props.theme.borders[2]}
    ${props => props.theme.colors.spotBackground[0]};
  border-radius: ${props => props.theme.radii[3]}px;
  background-color: ${props => getBackgroundColor(props)};

  ${p =>
    p.shouldDisplayWarning &&
    css`
      border: 2px solid ${p.theme.colors.interactive.solid.alert.default};
      background-color: ${getStatusBackgroundColor({
        showingStatusInfo: p.showingStatusInfo,
        theme: p.theme,
        action: '',
        viewType: 'card',
      })};
    `}

  ${p =>
    p.showingStatusInfo &&
    css`
      border: 2px solid ${p.theme.colors.interactive.solid.alert.active};
    `}

  &:hover {
    // Make the border invisible instead of removing it, this is to prevent things from shifting due to the size change.
    border: ${props => props.theme.borders[2]} rgba(0, 0, 0, 0);

    ${p =>
      p.shouldDisplayWarning &&
      css`
        border-color: ${p.theme.colors.interactive.solid.alert.hover};
        background-color: ${getStatusBackgroundColor({
          showingStatusInfo: p.showingStatusInfo,
          theme: p.theme,
          action: 'hover',
          viewType: 'card',
        })};
      `}
  }
`;

/**
 * The outer labels container is resized depending on whether we want to show a
 * single row, or all labels. It hides the internal container's overflow if more
 * than one row of labels exist, but is not yet visible.
 */
const LabelsContainer = styled(Box)<{ showAll?: boolean }>`
  ${props => (props.showAll ? '' : `height: ${labelRowHeight}px;`)}
  overflow: hidden;
`;

const StyledLabel = styled(Label)`
  height: ${labelHeight}px;
  margin: 1px 0;
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
