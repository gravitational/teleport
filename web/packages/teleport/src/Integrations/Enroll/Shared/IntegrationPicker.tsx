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

import { ReactNode, useMemo } from 'react';
import styled from 'styled-components';

import { Alert, Box, Flex, Indicator } from 'design';
import * as Icons from 'design/Icon';

import { FeatureHeader, FeatureHeaderTitle } from 'teleport/components/Layout';
import { TextIcon } from 'teleport/Discover/Shared';

import { integrationTagOptions, type BaseIntegration } from './common';
import { FilterPanel } from './FilterPanel';
import { filterIntegrations } from './filters';
import { useIntegrationPickerState } from './state';

function titleOrName<T extends BaseIntegration>(i: T) {
  if ('title' in i) {
    return i.title;
  } else if ('name' in i) {
    return i.name;
  }
}

export function displayName<T extends BaseIntegration>(i: T) {
  const name = titleOrName(i);
  if ('type' in i && i.type === 'bot') {
    return `Machine ID: ${name}`;
  }
  return name;
}

export function sortByDisplayName<T extends BaseIntegration>(a: T, b: T) {
  return displayName(a).localeCompare(displayName(b));
}

export interface IntegrationPickerProps<T extends BaseIntegration> {
  integrations: T[];
  renderIntegration: (integration: T) => ReactNode;
  initialSort?: (a: T, b: T) => number;
  isLoading?: boolean;
  canCreate?: boolean;
  ErrorMessage?: ReactNode;
}

export function IntegrationPicker<T extends BaseIntegration>({
  integrations,
  renderIntegration,
  initialSort = sortByDisplayName,
  isLoading,
  canCreate,
  ErrorMessage,
}: IntegrationPickerProps<T>) {
  const [state, setState] = useIntegrationPickerState();

  const activeTagOptions = integrationTagOptions.filter(tagOption =>
    integrations.some(integration => integration.tags.includes(tagOption.value))
  );

  const sortedIntegrations = useMemo(() => {
    const sorted = integrations.toSorted((a, b) => {
      switch (state.sortKey) {
        case 'name':
          return sortByDisplayName(a, b);
        default:
          return initialSort(a, b);
      }
    });

    if (state.sortDirection === 'DESC') {
      sorted.reverse();
    }

    return sorted;
  }, [integrations, state.sortDirection, state.sortKey]);

  const filteredIntegrations = useMemo(
    () =>
      filterIntegrations(sortedIntegrations, state.filters.tags, state.search),
    [sortedIntegrations, state.filters.tags, state.search]
  );

  const renderPermissionsNotice = () => {
    if (!canCreate) {
      return (
        <Alert kind="info" mt={4}>
          <Flex gap={2}>
            You do not have permission to create Integrations. You must have at
            least one of these role permissions: <code>plugin.create</code>{' '}
            <code>integration.create</code>
          </Flex>
        </Alert>
      );
    }
  };

  const renderIntegrations = () => {
    if (isLoading) {
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    } else {
      return (
        <>
          <FilterPanel
            state={state}
            setState={setState}
            tagOptions={activeTagOptions}
          />
          {!filteredIntegrations.length && (
            <TextIcon>
              <Icons.Magnifier size="small" /> No results found
            </TextIcon>
          )}
          <Container role="grid">
            {filteredIntegrations.map(i => {
              return renderIntegration(i);
            })}
          </Container>
        </>
      );
    }
  };

  return (
    <>
      <Box>
        <FeatureHeader mb={0}>
          <FeatureHeaderTitle>Enroll a New Integration</FeatureHeaderTitle>
        </FeatureHeader>
      </Box>
      {renderPermissionsNotice()}

      <Flex flexDirection="column" gap={3}>
        {ErrorMessage ? ErrorMessage : renderIntegrations()}
      </Flex>
    </>
  );
}

const Container = styled.div`
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: ${props => props.theme.space[3]}px;
  margin-bottom: ${props => props.theme.space[4]}px;
`;
