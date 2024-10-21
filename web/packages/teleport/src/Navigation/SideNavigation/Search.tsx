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

import React, { useState, useRef } from 'react';
import { NavLink } from 'react-router-dom';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { height, space, color } from 'design/system';

import { NavigationSection, NavigationSubsection } from './Navigation';
import {
  Section,
  RightPanel,
  verticalPadding,
  getSubsectionStyles,
} from './Section';
import { CategoryIcon } from './CategoryIcon';
import { CustomNavigationCategory } from './categories';

export function SearchSection({
  navigationSections,
  expandedSection,
  previousExpandedSection,
  handleSetExpandedSection,
  currentView,
}: {
  navigationSections: NavigationSection[];
  expandedSection: NavigationSection;
  previousExpandedSection: NavigationSection;
  currentView: NavigationSubsection;
  handleSetExpandedSection: (section: NavigationSection) => void;
}) {
  const section: NavigationSection = {
    category: CustomNavigationCategory.Search,
    subsections: [],
  };

  const isExpanded =
    expandedSection?.category === CustomNavigationCategory.Search;
  return (
    <Section
      key="search"
      section={section}
      $active={false}
      onClick={() => null}
      setExpandedSection={() => handleSetExpandedSection(section)}
      aria-controls={`panel-${expandedSection?.category}`}
      isExpanded={isExpanded}
    >
      <RightPanel
        isVisible={isExpanded}
        skipAnimation={!!previousExpandedSection}
        id={`panel-${section.category}`}
        onFocus={() => handleSetExpandedSection(section)}
      >
        <SearchContent
          navigationSections={navigationSections}
          currentView={currentView}
        />
      </RightPanel>
    </Section>
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

  return (
    <Flex flexDirection="column">
      <Flex py={verticalPadding} px={3}>
        <Text typography="h2" color="text.slightlyMuted">
          Search
        </Text>
      </Flex>
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
                $active={currentView?.route === subsection.route}
              />
            ))}
          </Flex>
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
    <SearchResultWrapper as={NavLink} to={subsection.route} $active={$active}>
      <Flex width="100%" gap={2} alignItems="start">
        <Box pt={1}>
          <CategoryIcon
            category={subsection.category}
            size={20}
            color="text.slightlyMuted"
          />
        </Box>
        <Flex flexDirection="column" alignItems="start">
          <Text typography="body2" color="text.slightlyMuted">
            {subsection.title}
          </Text>
          <Text typography="body3" color="text.muted">
            {subsection.category}
          </Text>
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
