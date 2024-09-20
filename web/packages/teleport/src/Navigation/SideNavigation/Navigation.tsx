/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useState, useRef, useCallback, useEffect } from 'react';
import styled, { css } from 'styled-components';
import { matchPath, useHistory } from 'react-router';
import { NavLink } from 'react-router-dom';
import { Text, Flex, Box } from 'design';
import { Theme } from 'design/theme/themes/types';

import cfg from 'teleport/config';

import { useFeatures } from 'teleport/FeaturesContext';
import { zIndexMap } from 'teleport/Main';

import { NavigationCategory, NAVIGATION_CATEGORIES } from './categories';
import { CategoryIcon } from './CategoryIcon';

import type * as history from 'history';

import type { TeleportFeature } from 'teleport/types';

const SideNavContainer = styled(Flex).attrs({
  gap: 2,
  pt: 2,
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'start',
  bg: 'levels.surface',
})`
  height: 100%;
  width: var(--sidenav-width);
  position: relative;
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};
  z-index: ${zIndexMap.sideNavContainer};
  overflow-y: auto;
`;

const verticalPadding = '12px';

const rightPanelWidth = '224px';

const RightPanel = styled(Box).attrs({ pt: 2, pb: 4, px: 2 })<{
  isVisible: boolean;
}>`
  position: absolute;
  top: 0;
  left: var(--sidenav-width);
  height: 100%;
  scrollbar-gutter: auto;
  overflow-y: auto;
  width: ${rightPanelWidth};
  background: ${p => p.theme.colors.levels.surface};
  z-index: ${zIndexMap.sideNavExpandedPanel};
  border-right: 1px solid ${p => p.theme.colors.spotBackground[1]};

  transition: transform 0.3s ease-in-out;
  ${props =>
    props.isVisible
      ? `
      transition: transform .15s ease-out;
      transform: translateX(0);
      `
      : `
      transition: transform .15s ease-in;
      transform: translateX(-100%);
      `}

  padding-top: ${p => p.theme.topBarHeight[0]}px;
  @media screen and (min-width: ${p => p.theme.breakpoints.small}px) {
    padding-top: ${p => p.theme.topBarHeight[1]}px;
  }
  @media screen and (min-width: ${p => p.theme.breakpoints.large}px) {
    padding-top: ${p => p.theme.topBarHeight[2]}px;
  }
`;

type NavigationSection = {
  category: NavigationCategory;
  subsections: NavigationSubsection[];
};

type NavigationSubsection = {
  category: NavigationCategory;
  title: string;
  route: string;
  exact: boolean;
  icon: (props) => JSX.Element;
  parent?: TeleportFeature;
};

function getNavigationSections(
  features: TeleportFeature[]
): NavigationSection[] {
  const navigationSections = NAVIGATION_CATEGORIES.map(category => ({
    category,
    subsections: getSubsectionsForCategory(category, features),
  }));

  return navigationSections;
}

function getSubsectionsForCategory(
  category: NavigationCategory,
  features: TeleportFeature[]
): NavigationSubsection[] {
  const filteredFeatures = features.filter(
    feature =>
      feature.sideNavCategory === category &&
      !!feature.navigationItem &&
      !feature.parent
  );

  return filteredFeatures.map(feature => {
    return {
      category,
      title: feature.navigationItem.title,
      route: feature.navigationItem.getLink(cfg.proxyCluster),
      exact: feature.navigationItem.exact,
      icon: feature.navigationItem.icon,
    };
  });
}

function getNavSubsectionForRoute(
  features: TeleportFeature[],
  route: history.Location<unknown> | Location
): NavigationSubsection {
  const feature = features
    .filter(feature => Boolean(feature.route))
    .find(feature =>
      matchPath(route.pathname, {
        path: feature.route.path,
        exact: false,
      })
    );

  if (!feature || !feature.sideNavCategory) {
    console.log('TRUE');
    return;
  }

  return {
    category: feature.sideNavCategory,
    title: feature.navigationItem.title,
    route: feature.navigationItem.getLink(cfg.proxyCluster),
    exact: feature.navigationItem.exact,
    icon: feature.navigationItem.icon,
  };
}

export function Navigation() {
  const features = useFeatures();
  const history = useHistory();
  const [expandedSection, setExpandedSection] =
    useState<NavigationSection | null>(null);
  const [expandedSectionIndex, setExpandedSectionIndex] = useState<number>(-1);

  const currentView = getNavSubsectionForRoute(features, history.location);

  const navSections = getNavigationSections(features).filter(
    section => section.subsections.length
  );

  const sectionRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const subsectionRefs = useRef<Array<HTMLAnchorElement | null>>([]);

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent, index: number, isSubsection: boolean) => {
      if (event.key === 'Tab') {
        if (!isSubsection) return;

        // When the user presses shift+tab on first subsection item of a section.
        if (event.shiftKey && index === 0) {
          event.preventDefault();
          const prevSectionIndex =
            (expandedSectionIndex - 1 + navSections.length) %
            navSections.length;
          sectionRefs.current[prevSectionIndex]?.focus();
        } else if (
          !event.shiftKey &&
          index === expandedSection.subsections.length - 1
        ) {
          // When the user presses tab on last subsection item of a section.
          event.preventDefault();
          const nextSectionIndex =
            (expandedSectionIndex + 1) % navSections.length;
          sectionRefs.current[nextSectionIndex]?.focus();
        }
      } else if (event.key === 'Enter' && !isSubsection) {
        event.preventDefault();
        subsectionRefs.current[0]?.focus();
      }
    },
    [expandedSection, expandedSectionIndex, navSections.length]
  );

  const handleSetExpandedSection = useCallback(
    (section: NavigationSection, index: number) => {
      setExpandedSection(section);
      setExpandedSectionIndex(index);
    },
    []
  );

  // Reset subsectionRefs when expanded section changes
  useEffect(() => {
    subsectionRefs.current = subsectionRefs.current.slice(
      0,
      expandedSection?.subsections.length || 0
    );
  }, [expandedSection]);

  return (
    <Box
      onMouseLeave={() => setExpandedSection(null)}
      onKeyUp={e => e.key === 'Escape' && setExpandedSection(null)}
      css={'height: 100%;'}
    >
      <SideNavContainer>
        {navSections.map((section, index) => (
          <Section
            key={section.category}
            section={section}
            active={section.category === currentView?.category}
            setExpandedSection={() => handleSetExpandedSection(section, index)}
            aria-controls={`panel-${expandedSection?.category}`}
            ref={el => (sectionRefs.current[index] = el)}
            onKeyDown={e => handleKeyDown(e, index, false)}
          />
        ))}
      </SideNavContainer>
      <RightPanel
        isVisible={!!expandedSection}
        id={`panel-${expandedSection?.category}`}
      >
        <Flex py={verticalPadding} px={3}>
          <Text typography="h2" color="text.slightlyMuted">
            {expandedSection?.category}
          </Text>
        </Flex>
        {expandedSection?.subsections.map((section, idx) => (
          <SubsectionItem
            ref={el => (subsectionRefs.current[idx] = el)}
            active={currentView?.route === section.route}
            to={section.route}
            exact={section.exact}
            key={section.title}
            tabIndex={0}
            role="button"
            onKeyDown={e => handleKeyDown(e, idx, true)}
          >
            <section.icon size={16} />
            <Text typography="body2">{section.title}</Text>
          </SubsectionItem>
        ))}
      </RightPanel>
    </Box>
  );
}

const SubsectionItem = React.forwardRef<
  HTMLAnchorElement,
  {
    active: boolean;
    to: string;
    exact: boolean;
    tabIndex: number;
    role: string;
    onKeyDown: (event: React.KeyboardEvent) => void;
    children: React.ReactNode;
  }
>((props, ref) => <StyledSubsectionItem ref={ref} {...props} />);

const StyledSubsectionItem = styled(NavLink)<{ active: boolean }>`
  display: flex;
  position: relative;
  color: ${props => props.theme.colors.text.slightlyMuted};
  text-decoration: none;
  user-select: none;
  gap: ${props => props.theme.space[2]}px;
  padding-top: ${verticalPadding};
  padding-bottom: ${verticalPadding};
  padding-left: ${props => props.theme.space[3]}px;
  padding-right: ${props => props.theme.space[3]}px;
  border-radius: ${props => props.theme.radii[2]}px;
  cursor: pointer;

  ${props => getSubsectionStyles(props.theme, props.active)}
`;

const Section = React.forwardRef<
  HTMLButtonElement,
  {
    section: NavigationSection;
    active: boolean;
    setExpandedSection: () => void;
    onKeyDown: (event: React.KeyboardEvent) => void;
  }
>(({ section, active, setExpandedSection, onKeyDown }, ref) => {
  return (
    <CategoryButton
      ref={ref}
      active={active}
      onMouseEnter={setExpandedSection}
      onFocus={setExpandedSection}
      onKeyDown={onKeyDown}
    >
      <CategoryIcon category={section.category} />
      {section.category}
    </CategoryButton>
  );
});

const CategoryButton = styled.button<{ active: boolean }>`
  height: 60px;
  width: 60px;
  cursor: pointer;
  outline: hidden;
  border: none;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border-radius: ${props => props.theme.radii[2]}px;

  font-size: ${props => props.theme.typography.body4.fontSize};
  font-weight: ${props => props.theme.typography.body4.fontWeight};
  letter-spacing: ${props => props.theme.typography.body4.letterSpacing};
  line-height: ${props => props.theme.typography.body4.lineHeight};

  ${props => getCategoryStyles(props.theme, props.active)}
`;

function getCategoryStyles(theme: Theme, active: boolean) {
  if (active) {
    return css`
      color: ${theme.colors.brand};
      background: ${theme.colors.interactive.tonal.primary[0].background};
      &:hover,
      &:focus {
        background: ${theme.colors.interactive.tonal.primary[1].background};
        color: ${theme.colors.interactive.tonal.primary[0].text};
      }
      &:active {
        background: ${theme.colors.interactive.tonal.primary[2].background};
        color: ${theme.colors.interactive.tonal.primary[1].text};
      }
    `;
  }

  return css`
    background: transparent;
    color: ${props => props.theme.colors.text.slightlyMuted};
    &:hover,
    &:focus {
      background: ${props => props.theme.colors.interactive.tonal.neutral[0]};
      color: ${props => props.theme.colors.text.main};
    }
    &:active {
      background: ${props => props.theme.colors.interactive.tonal.neutral[1]};
      color: ${props => props.theme.colors.text.main};
    }
  `;
}

function getSubsectionStyles(theme: Theme, active: boolean) {
  if (active) {
    return css`
      color: ${theme.colors.brand};
      background: ${theme.colors.interactive.tonal.primary[0].background};
      &:focus {
        outline: 2px solid
          ${theme.colors.interactive.solid.primary.default.background};
      }
      &:hover {
        background: ${theme.colors.interactive.tonal.primary[1].background};
        color: ${theme.colors.interactive.tonal.primary[0].text};
      }
      &:active {
        background: ${theme.colors.interactive.tonal.primary[2].background};
        color: ${theme.colors.interactive.tonal.primary[1].text};
      }
    `;
  }

  return css`
    color: ${props => props.theme.colors.text.slightlyMuted};
    &:focus {
      outline: 2px solid ${theme.colors.text.muted};
    }
    &:hover {
      background: ${props => props.theme.colors.interactive.tonal.neutral[0]};
      color: ${props => props.theme.colors.text.main};
    }
    &:active {
      background: ${props => props.theme.colors.interactive.tonal.neutral[1]};
      color: ${props => props.theme.colors.text.main};
    }
  `;
}

// TODO(rudream): Figure out best place for for license footers.
// function AGPLFooter() {
//   return (
//     <LicenseFooter
//       title="AGPL Edition"
//       subText="Unofficial Version"
//       infoContent={
//         <>
//           {/* This is an independently compiled AGPL-3.0 version of Teleport. You */}
//           {/* can find the official release on{' '} */}
//           This is an independently compiled AGPL-3.0 version of Teleport.
//           <br />
//           Visit{' '}
//           <Text
//             as="a"
//             href="https://goteleport.com/download/?utm_source=oss&utm_medium=in-product&utm_campaign=limited-features"
//             target="_blank"
//           >
//             the Downloads page
//           </Text>{' '}
//           for the official release.
//         </>
//       }
//     />
//   );
// }

// function CommunityFooter() {
//   return (
//     <LicenseFooter
//       title="Community Edition"
//       subText="Limited Features"
//       infoContent={
//         <>
//           <Text
//             as="a"
//             href="https://goteleport.com/signup/enterprise/?utm_source=oss&utm_medium=in-product&utm_campaign=limited-features"
//             target="_blank"
//           >
//             Upgrade to Teleport Enterprise
//           </Text>{' '}
//           for SSO, just-in-time access requests, Access Graph, and much more!
//         </>
//       }
//     />
//   );
// }

// function LicenseFooter({
//   title,
//   subText,
//   infoContent,
// }: {
//   title: string;
//   subText: string;
//   infoContent: JSX.Element;
// }) {
//   const [opened, setOpened] = useState(false);
//   return (
//     <StyledFooterBox py={3} px={4} onMouseLeave={() => setOpened(false)}>
//       <Flex alignItems="center" gap={2}>
//         <Text>{title}</Text>
//         <FooterContent onMouseEnter={() => setOpened(true)}>
//           <Icons.Info size={16} />
//           {opened && <TooltipContent>{infoContent}</TooltipContent>}
//         </FooterContent>
//       </Flex>
//       <SubText>{subText}</SubText>
//     </StyledFooterBox>
//   );
// }

// const StyledFooterBox = styled(Box)`
//   line-height: 20px;
//   border-top: ${props => props.theme.borders[1]}
//     ${props => props.theme.colors.spotBackground[0]};
// `;

// const SubText = styled(Text)`
//   color: ${props => props.theme.colors.text.disabled};
//   font-size: ${props => props.theme.fontSizes[1]}px;
// `;

// const TooltipContent = styled(Box)`
//   width: max-content;
//   position: absolute;
//   bottom: 0;
//   left: 24px;
//   padding: 12px 16px 12px 16px;
//   box-shadow: ${p => p.theme.boxShadow[1]};
//   background-color: ${props => props.theme.colors.tooltip.background};
//   z-index: 20;
// `;

// const FooterContent = styled(Flex)`
//   position: relative;
// `;
