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

import React, { useEffect, useRef, useState } from 'react';
import { NavLink } from 'react-router-dom';
import styled from 'styled-components';

import { Box, Flex, P3, Text } from 'design';
import { color, height, space } from 'design/system';

import { storageService } from 'teleport/services/storageService';

import { CustomNavigationCategory } from './categories';
import {
  NavigationSection,
  NavigationSubsection,
  useFloatingUiWithRestMs,
} from './Navigation';
import { RecentHistory, RecentHistoryItem } from './RecentHistory';
import {
  CustomChildrenSection,
  getSubsectionStyles,
  RightPanel,
  RightPanelHeader,
} from './Section';

export function SearchSection({
  navigationSections,
  expandedSection,
  previousExpandedSection,
  handleSetExpandedSection,
  currentView,
  stickyMode,
  toggleStickyMode,
  canToggleStickyMode,
}: {
  navigationSections: NavigationSection[];
  expandedSection: NavigationSection;
  previousExpandedSection: NavigationSection;
  currentView: NavigationSubsection;
  handleSetExpandedSection: (section: NavigationSection) => void;
  stickyMode: boolean;
  toggleStickyMode: () => void;
  canToggleStickyMode: boolean;
}) {
  const section: NavigationSection = {
    category: CustomNavigationCategory.Search,
    subsections: [],
  };

  const isExpanded =
    expandedSection?.category === CustomNavigationCategory.Search;

  const { refs, getReferenceProps, getFloatingProps } = useFloatingUiWithRestMs(
    {
      open: isExpanded,
      onOpenChange: open => open && handleSetExpandedSection(section),
    }
  );

  return (
    <CustomChildrenSection
      ref={refs.setReference}
      key="search"
      section={section}
      $active={false}
      aria-controls={`panel-${expandedSection?.category}`}
      isExpanded={isExpanded}
      {...getReferenceProps()}
    >
      <RightPanel
        ref={refs.setFloating}
        isVisible={isExpanded}
        skipAnimation={!!previousExpandedSection}
        id={`panel-${section.category}`}
        onFocus={() => handleSetExpandedSection(section)}
        {...getFloatingProps()}
      >
        <Box
          css={`
            overflow-y: auto;
            padding: 3px;
          `}
        >
          <RightPanelHeader
            title={section.category}
            stickyMode={stickyMode}
            toggleStickyMode={toggleStickyMode}
            canToggleStickyMode={canToggleStickyMode}
          />
          <SearchContent
            navigationSections={navigationSections}
            currentView={currentView}
          />
        </Box>
      </RightPanel>
    </CustomChildrenSection>
  );
}

function SearchContent({
  navigationSections,
  currentView,
}: {
  navigationSections: NavigationSection[];
  currentView: NavigationSubsection;
}) {
  const [searchInput, setSearchInput] = useState('');
  const inputRef = useRef<HTMLInputElement>();

  const handleSearch = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchInput(e.target.value);
  };

  const results = navigationSections.flatMap(section =>
    section.subsections.filter(subsection =>
      subsection.searchableTags?.some(
        tag =>
          searchInput.length > 0 &&
          tag.toLowerCase().includes(searchInput.toLocaleLowerCase())
      )
    )
  );

  const [recentHistory, setRecentHistory] = useState<RecentHistoryItem[]>(
    storageService.getRecentHistory()
  );

  useEffect(() => {
    if (currentView) {
      const newRecentHistory = storageService.addRecentHistoryItem({
        category: currentView?.category,
        title: currentView?.title,
        route: currentView?.route,
        exact: currentView?.exact,
      });

      setRecentHistory(newRecentHistory);
    }
  }, [currentView]);

  function handleRemoveItem(route: string) {
    const newRecentHistory = storageService.removeRecentHistoryItem(route);
    setRecentHistory(newRecentHistory);
  }

  return (
    <Flex flexDirection="column">
      <Flex alignItems="center" flexDirection="column">
        <SearchInput
          type="text"
          onChange={handleSearch}
          value={searchInput}
          ref={inputRef}
          placeholder="Search for a page..."
        />
        {results.length > 0 && (
          <Flex flexDirection="column" mt={3} width="100%" gap={1}>
            {results.map((subsection, index) => (
              <SearchResult
                key={index}
                subsection={subsection}
                $active={
                  subsection?.customRouteMatchFn
                    ? subsection.customRouteMatchFn(currentView?.route)
                    : currentView?.route === subsection.route
                }
              />
            ))}
          </Flex>
        )}
        {searchInput.length === 0 && (
          <RecentHistory
            recentHistoryItems={recentHistory}
            onRemoveItem={handleRemoveItem}
          />
        )}
      </Flex>
    </Flex>
  );
}

function SearchResult({
  subsection,
  $active,
}: {
  subsection: NavigationSubsection;
  $active: boolean;
}) {
  return (
    <SearchResultWrapper
      as={NavLink}
      to={subsection.route}
      $active={$active}
      onClick={subsection.onClick}
    >
      <Flex width="100%" gap={2} alignItems="start">
        <Flex height="24px" alignItems="center" justifyContent="center">
          <subsection.icon size={20} color="text.slightlyMuted" />
        </Flex>
        <Flex flexDirection="column" alignItems="start">
          <Text typography="body2" color="text.slightlyMuted">
            {subsection.title}
          </Text>
          {subsection.category && (
            <P3 color="text.muted">{subsection.category}</P3>
          )}
        </Flex>
      </Flex>
    </SearchResultWrapper>
  );
}

const SearchResultWrapper = styled(Box)<{ $active: boolean }>`
  padding: ${props => props.theme.space[2]}px ${props => props.theme.space[3]}px;
  text-decoration: none;
  user-select: none;
  border-radius: ${props => props.theme.radii[2]}px;

  ${props => getSubsectionStyles(props.theme, props.$active)}
`;

const SearchInput = styled.input`
  border: none;
  outline: none;
  box-sizing: border-box;
  font-size: ${props => props.theme.fontSizes[2]}px;
  height: 32px;
  width: 192px;
  ${color}
  ${space}
  ${height}
  color: ${props => props.theme.colors.text.main};
  background: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => props.theme.space[3]}px;
  border-radius: 29px;
`;
