/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useCallback, useState, useLayoutEffect, useRef } from 'react';
import styled from 'styled-components';

import {
  Box,
  ButtonIcon,
  ButtonLink,
  Flex,
  Label,
  Popover,
  Text,
} from 'design';
import copyToClipboard from 'design/utils/copyToClipboard';

import { ResourceIcon, ResourceIconName } from 'design/ResourceIcon';
import {
  Copy,
  Check,
  Application as ApplicationsIcon,
  Database as DatabasesIcon,
  Kubernetes as KubernetesIcon,
  Server as ServersIcon,
  Desktop as DesktopsIcon,
} from 'design/Icon';

import {
  ResourceLabel,
  UnifiedResource,
  UnifiedResourceKind,
} from 'teleport/services/agents';
import { Database } from 'teleport/services/databases';

import { ResourceActionButton } from './ResourceActionButton';

// Since we do a lot of manual resizing and some absolute positioning, we have
// to put some layout constants in place here.
const labelRowHeight = 20; // px
const labelVerticalMargin = 1; // px
const labelHeight = labelRowHeight * labelVerticalMargin;

/**
 * This box serves twofold purpose: first, it prevents the underlying icon from
 * being squeezed if the parent flexbox starts shrinking items. Second, it
 * prevents the icon from magically occupying too much space, since the SVG
 * element somehow forces the parent to occupy at least full line height.
 */
const ResTypeIconBox = styled(Box)`
  line-height: 0;
`;

type Props = {
  resource: UnifiedResource;
  onLabelClick?: (label: ResourceLabel) => void;
};

export function ResourceCard({ resource, onLabelClick }: Props) {
  const name = resourceName(resource);
  const resIcon = resourceIconName(resource);
  const ResTypeIcon = resourceTypeIcon(resource.kind);
  const description = resourceDescription(resource);

  const labelsInnerContainer = useRef(null);

  const [showMoreLabelsButton, setShowMoreLabelsButton] = useState(false);
  const [showAllLabels, setShowAllLabels] = useState(false);
  const [numMoreLabels, setNumMoreLabels] = useState(0);

  const nameTextRef = useRef<HTMLDivElement | null>(null);
  const [isNameOverflowed, setIsNameOverflowed] = useState(false);
  // This effect installs a resize observer whose purpose is to detect the size
  // of the component that contains all the labels. If this component is taller
  // than the height of a single label row, we show a "+x more" button.
  useLayoutEffect(() => {
    if (!labelsInnerContainer.current) return;

    const observer = new ResizeObserver(entries => {
      // This check will let us know if the name text has overflowed. We do this
      // to conditionally render a tooltip for only overflowed names
      if (
        nameTextRef.current?.scrollWidth >
        nameTextRef.current?.parentElement.offsetWidth
      ) {
        setIsNameOverflowed(true);
      } else {
        setIsNameOverflowed(false);
      }
      const container = entries[0];

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
  });

  const onMoreLabelsClick = () => {
    setShowAllLabels(true);
  };

  return (
    <CardContainer>
      <CardInnerContainer
        p={3}
        alignItems="start"
        showAllLabels={showAllLabels}
        onMouseLeave={() => setShowAllLabels(false)}
      >
        <ResourceIcon name={resIcon} width="45px" height="45px" ml={2} />
        {/* MinWidth is important to prevent descriptions from overflowing. */}
        <Flex flexDirection="column" flex="1" minWidth="0" ml={3} gap={1}>
          <Flex flexDirection="row" alignItems="center">
            <SingleLineBox flex="1">
              {isNameOverflowed ? (
                <HoverTooltip tipContent={<>{name}</>}>
                  <Text ref={nameTextRef} typography="h5" fontWeight={300}>
                    {name}
                  </Text>
                </HoverTooltip>
              ) : (
                <Text ref={nameTextRef} typography="h5" fontWeight={300}>
                  {name}
                </Text>
              )}
            </SingleLineBox>
            <CopyButton name={name} />
            <ResourceActionButton resource={resource} />
          </Flex>
          <Flex flexDirection="row" alignItems="center">
            <ResTypeIconBox>
              <ResTypeIcon size={18} />
            </ResTypeIconBox>
            {description.primary && (
              <SingleLineBox ml={1} title={description.primary}>
                <Text typography="body2" color="text.slightlyMuted">
                  {description.primary}
                </Text>
              </SingleLineBox>
            )}
            {description.secondary && (
              <SingleLineBox ml={2} title={description.secondary}>
                <Text typography="body2" color="text.muted">
                  {description.secondary}
                </Text>
              </SingleLineBox>
            )}
          </Flex>
          <LabelsContainer showAll={showAllLabels}>
            <LabelsInnerContainer ref={labelsInnerContainer}>
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
              {resource.labels.map((label, i) => {
                const { name, value } = label;
                const labelText = `${name}: ${value}`;
                return (
                  <ResourceLabel
                    key={JSON.stringify([name, value, i])}
                    title={labelText}
                    onClick={() => onLabelClick?.(label)}
                    kind="secondary"
                    data-is-label=""
                  >
                    {labelText}
                  </ResourceLabel>
                );
              })}
            </LabelsInnerContainer>
          </LabelsContainer>
        </Flex>
      </CardInnerContainer>
    </CardContainer>
  );
}

function CopyButton({ name }: { name: string }) {
  const copySuccess = 'Copied!';
  const copyDefault = 'Click to copy';
  const copyAnchorEl = useRef(null);
  const [copiedText, setCopiedText] = useState(copyDefault);

  const handleCopy = useCallback(() => {
    setCopiedText(copySuccess);
    copyToClipboard(name);
    // Change to default text after 1 second
    setTimeout(() => {
      setCopiedText(copyDefault);
    }, 1000);
  }, [name]);

  return (
    <HoverTooltip tipContent={<>{copiedText}</>}>
      <ButtonIcon setRef={copyAnchorEl} size={0} mr={2} onClick={handleCopy}>
        {copiedText === copySuccess ? (
          <Check size="small" />
        ) : (
          <Copy size="small" />
        )}
      </ButtonIcon>
    </HoverTooltip>
  );
}

function resourceName(resource: UnifiedResource) {
  return resource.kind === 'node' ? resource.hostname : resource.name;
}

function resourceDescription(resource: UnifiedResource) {
  switch (resource.kind) {
    case 'app':
      return {
        primary: resource.addrWithProtocol,
        secondary: resource.description,
      };
    case 'db':
      return { primary: resource.type, secondary: resource.description };
    case 'kube_cluster':
      return { primary: 'Kubernetes' };
    case 'node':
      return {
        primary: resource.subKind || 'SSH Server',
        secondary: resource.tunnel ? '' : resource.addr,
      };
    case 'windows_desktop':
      return { primary: 'Windows', secondary: resource.addr };

    default:
      return {};
  }
}

function databaseIconName(resource: Database): ResourceIconName {
  switch (resource.protocol) {
    case 'postgres':
      return 'Postgres';
    case 'mysql':
      return 'MysqlLarge';
    case 'mongodb':
      return 'Mongo';
    case 'cockroachdb':
      return 'Cockroach';
    case 'snowflake':
      return 'Snowflake';
    case 'dynamodb':
      return 'Dynamo';
    default:
      return 'Database';
  }
}

function resourceIconName(resource: UnifiedResource): ResourceIconName {
  switch (resource.kind) {
    case 'app':
      return resource.guessedAppIconName;
    case 'db':
      return databaseIconName(resource);
    case 'kube_cluster':
      return 'Kube';
    case 'node':
      return 'Server';
    case 'windows_desktop':
      return 'Windows';

    default:
      return 'Server';
  }
}

function resourceTypeIcon(kind: UnifiedResourceKind) {
  switch (kind) {
    case 'app':
      return ApplicationsIcon;
    case 'db':
      return DatabasesIcon;
    case 'kube_cluster':
      return KubernetesIcon;
    case 'node':
      return ServersIcon;
    case 'windows_desktop':
      return DesktopsIcon;

    default:
      return ServersIcon;
  }
}

/**
 * The outer container's purpose is to reserve horizontal space on the resource
 * grid. It holds the inner container that normally holds a regular layout of
 * the card, and is fully contained inside the outer container.  Once the user
 * clicks the "more" button, the inner container "pops out" by changing its
 * position to absolute.
 *
 * TODO(bl-nero): Known issue: this doesn't really work well with one-column
 * layout; we may need to globally set the card height to fixed size on the
 * outer container.
 */
const CardContainer = styled(Box)`
  position: relative;
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
const CardInnerContainer = styled(Flex)`
  border-top: 2px solid ${props => props.theme.colors.spotBackground[0]};
  background-color: transparent;

  ${props =>
    props.showAllLabels
      ? 'position: absolute; left: 0; right: 0; z-index: 1;'
      : ''}

  transition: all 150ms;

  ${CardContainer}:hover & {
    background-color: ${props => props.theme.colors.levels.elevated};
    border-color: ${props => props.theme.colors.levels.elevated};
    box-shadow: ${props => props.theme.boxShadow[1]};
  }

  @media (min-width: ${props => props.theme.breakpoints.tablet}px) {
    border: ${props => props.theme.borders[2]}
      ${props => props.theme.colors.spotBackground[0]};
    border-radius: ${props => props.theme.radii[3]}px;
  }
`;

const SingleLineBox = styled(Box)`
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
`;

export const HoverTooltip: React.FC<{
  tipContent: React.ReactElement;
  fontSize?: number;
}> = ({ tipContent, fontSize = 10, children }) => {
  const [anchorEl, setAnchorEl] = useState();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event) {
    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(null);
  }

  return (
    <>
      <span
        aria-owns={open ? 'mouse-over-popover' : undefined}
        onMouseEnter={handlePopoverOpen}
        onMouseLeave={handlePopoverClose}
      >
        {children}
      </span>
      <Popover
        modalCss={modalCss}
        onClose={handlePopoverClose}
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: 'top',
          horizontal: 'center',
        }}
        transformOrigin={{
          vertical: 'bottom',
          horizontal: 'center',
        }}
      >
        <StyledOnHover px={2} py={1} fontSize={`${fontSize}px`}>
          {tipContent}
        </StyledOnHover>
      </Popover>
    </>
  );
};

const modalCss = () => `
  pointer-events: none;
`;

const StyledOnHover = styled(Text)`
  color: ${props => props.theme.colors.text.main};
  background-color: ${props => props.theme.colors.tooltip.background};
  max-width: 350px;
`;

/**
 * The outer labels container is resized depending on whether we want to show a
 * single row, or all labels. It hides the internal container's overflow if more
 * than one row of labels exist, but is not yet visible.
 */
const LabelsContainer = styled(Box)`
  ${props => (props.showAll ? '' : `height: ${labelRowHeight}px;`)}
  overflow: hidden;
`;

const ResourceLabel = styled(Label)`
  height: ${labelHeight}px;
  margin: 1px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
`;

/**
 * The inner labels container always adapts to the size of labels.  Its height
 * is measured by the resize observer.
 */
const LabelsInnerContainer = styled(Flex)`
  position: relative;
  flex-wrap: wrap;
  align-items: start;
  gap: ${props => props.theme.space[1]}px;
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

  background-color: ${props => props.theme.colors.levels.sunken};
  color: ${props => props.theme.colors.text.slightlyMuted};
  font-style: italic;
  border-radius: 0;

  transition: visibility 0s;
  transition: background 150ms;

  .grv-unified-resource-card:hover & {
    background-color: ${props => props.theme.colors.levels.elevated};
  }
`;
