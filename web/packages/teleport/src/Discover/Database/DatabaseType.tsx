import React from 'react';
import styled from 'styled-components';

import { Flex } from 'design';
import { Cloud, Database as DatabaseIcon } from 'design/Icon';

import {
  Database,
  DatabaseLocation,
} from 'teleport/Discover/Database/resources';

interface DatabaseTypeProps {
  database: Database;
  selected: boolean;
  onSelect: () => void;
}

interface DatabaseTypeContainerProps {
  selected: boolean;
}

const PopularBadge = styled.div`
  color: white;
  border-radius: 8px;
  font-size: 12px;
  padding: 4px 6px;
  line-height: 1;
`;

const DatabaseTypeContainer = styled('div')<DatabaseTypeContainerProps>`
  background: ${p => (p.selected ? '#5130c9' : '#1d2752')};
  border-radius: 8px;
  padding: 12px 12px;
  color: white;
  cursor: pointer;

  ${PopularBadge} {
    background: ${p => (p.selected ? '#4126a1' : '#512fc9')};
  }
`;

const DatabaseName = styled.h3`
  margin: 10px 0 0 0;
  font-size: 14px;
  font-weight: 700;
`;

export function DatabaseType(props: DatabaseTypeProps) {
  let popular;
  if (props.database.popular) {
    popular = <PopularBadge>popular</PopularBadge>;
  }

  return (
    <DatabaseTypeContainer
      selected={props.selected}
      onClick={() => props.onSelect()}
    >
      <Flex justifyContent="space-between" alignItems="center">
        {getDatabaseIcon(props.database)}

        {popular}
      </Flex>

      <DatabaseName>{props.database.name}</DatabaseName>
    </DatabaseTypeContainer>
  );
}

function getDatabaseIcon(database: Database) {
  switch (database.location) {
    case DatabaseLocation.AWS:
    case DatabaseLocation.GCP:
      return <Cloud fontSize={22} />;
    case DatabaseLocation.SelfHosted:
      return <DatabaseIcon fontSize={22} />;
    default:
      return <DatabaseIcon fontSize={22} />;
  }
}
