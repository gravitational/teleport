import React, { useState } from 'react';
import styled from 'styled-components';

import { Box, Flex } from 'design';

import SlideTabs from 'design/SlideTabs';

import { useDiscover } from 'teleport/Discover/useDiscover';
import {
  Database,
  DatabaseLocation,
  DATABASES,
} from 'teleport/Discover/Database/resources';
import { DatabaseType } from 'teleport/Discover/Database/DatabaseType';

import { Header } from '../Shared';

const Databases = styled.div`
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  column-gap: 10px;
  row-gap: 15px;
`;

enum FilterType {
  All = 'All',
  AWS = 'AWS',
  GCP = 'GCP',
  SelfHosted = 'Self-Hosted',
}

const FILTER_TYPES = [
  FilterType.All,
  FilterType.AWS,
  FilterType.GCP,
  FilterType.SelfHosted,
];

export function SelectDatabaseType() {
  const { resourceState, setResourceState } = useDiscover<Database>();

  const [filter, setFilter] = useState<FilterType>(FilterType.All);

  const databases = [];
  for (const [index, database] of DATABASES.entries()) {
    switch (filter) {
      case FilterType.SelfHosted:
        if (database.location !== DatabaseLocation.SelfHosted) {
          continue;
        }

        break;
      case FilterType.AWS:
        if (database.location !== DatabaseLocation.AWS) {
          continue;
        }

        break;
      case FilterType.GCP:
        if (database.location !== DatabaseLocation.GCP) {
          continue;
        }

        break;
    }

    databases.push(
      <DatabaseType
        database={database}
        key={index}
        selected={database === resourceState}
        onSelect={() => setResourceState(database)}
      />
    );
  }

  return (
    <Box mt={6}>
      <Flex alignItems="center" justifyContent="space-between" mb={4}>
        <Header>Select Deployment Type</Header>

        <Box width="470px">
          <SlideTabs
            appearance="round"
            size="medium"
            tabs={FILTER_TYPES}
            onChange={index => setFilter(FILTER_TYPES[index])}
          />
        </Box>
      </Flex>
      <Databases>{databases}</Databases>
    </Box>
  );
}
