import { ReactNode, useMemo } from 'react';
import styled from 'styled-components';

import { Alert, Box, Flex, Indicator } from 'design';
import * as Icons from 'design/Icon';

import { FeatureHeader, FeatureHeaderTitle } from 'teleport/components/Layout';
import { TextIcon } from 'teleport/Discover/Shared';
import { filterIntegrations } from 'teleport/Integrations/Enroll/utils/filters';
import { ResourceFilter } from 'teleport/services/agents';

import {
  integrationTagOptions,
  type BaseIntegration,
  type IntegrationTag,
} from './common';
import { FilterPanel } from './FilterPanel';

export function titleOrName<T extends BaseIntegration>(i: T) {
  if ('title' in i) {
    return i.title;
  } else if ('name' in i) {
    return i.name;
  }
}

export function sortByTitleOrName<T extends BaseIntegration>(a: T, b: T) {
  return titleOrName(a).localeCompare(titleOrName(b));
}

export interface IntegrationPickerProps<T extends BaseIntegration> {
  integrations: T[];
  renderIntegration: (integration: T) => ReactNode;
  params: ResourceFilter;
  setParams: (params: ResourceFilter) => void;
  initialSort?: (a: T, b: T) => number;
  isLoading?: boolean;
  canCreate?: boolean;
  ErrorMessage?: ReactNode;
}

export function IntegrationPicker<T extends BaseIntegration>({
  integrations,
  renderIntegration,
  params,
  setParams,
  initialSort = sortByTitleOrName,
  isLoading,
  canCreate,
  ErrorMessage,
}: IntegrationPickerProps<T>) {
  const sortedIntegrations = useMemo(() => {
    const sorted = integrations.toSorted((a, b) => {
      switch (params.sort?.fieldName) {
        case 'name':
          return sortByTitleOrName(a, b);
        default:
          return initialSort(a, b);
      }
    });

    if (params.sort?.dir === 'DESC') {
      sorted.reverse();
    }

    return sorted;
  }, [integrations, params.sort]);

  const filteredIntegrations = useMemo(
    () =>
      filterIntegrations(
        sortedIntegrations,
        (params.kinds as IntegrationTag[]) || [],
        params.search || ''
      ),
    [params.kinds, sortedIntegrations, params.search]
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
            params={params}
            setParams={setParams}
            integrationTagOptions={integrationTagOptions}
          />
          {!filteredIntegrations.length && (
            <TextIcon>
              <Icons.Magnifier size="small" /> No results found
            </TextIcon>
          )}
          <Box mb={4}>
            <Container role="grid">
              {filteredIntegrations.map(i => {
                return renderIntegration(i);
              })}
            </Container>
          </Box>
        </>
      );
    }
  };

  return (
    <>
      <Box my={3}>
        <FeatureHeader>
          <FeatureHeaderTitle>Enroll a New Integration</FeatureHeaderTitle>
        </FeatureHeader>
      </Box>
      {renderPermissionsNotice()}

      <Flex flexDirection="column" gap={4}>
        {ErrorMessage ? ErrorMessage : renderIntegrations()}
      </Flex>
    </>
  );
}

const Container = styled.div`
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 16px;
`;
