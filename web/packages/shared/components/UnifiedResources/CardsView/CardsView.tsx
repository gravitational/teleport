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

import styled from 'styled-components';

import { Flex } from 'design';

import { LoadingSkeleton } from '../shared/LoadingSkeleton';
import { ResourceViewProps } from '../types';
import { FETCH_MORE_SIZE } from '../UnifiedResources';
import { LoadingCard } from './LoadingCard';
import { ResourceCard } from './ResourceCard';

export function CardsView({
  mappedResources,
  onLabelClick,
  pinnedResources,
  selectedResources,
  onSelectResource,
  onPinResource,
  isProcessing,
  pinningSupport,
}: ResourceViewProps) {
  return (
    <CardsContainer className="CardsContainer" gap={2}>
      {mappedResources.map(({ item, key }) => (
        <ResourceCard
          key={key}
          name={item.name}
          ActionButton={item.ActionButton}
          primaryIconName={item.primaryIconName}
          onLabelClick={onLabelClick}
          SecondaryIcon={item.SecondaryIcon}
          cardViewProps={item.cardViewProps}
          labels={item.labels}
          pinned={pinnedResources.includes(key)}
          requiresRequest={item.requiresRequest}
          pinningSupport={pinningSupport}
          selected={selectedResources.includes(key)}
          selectResource={() => onSelectResource(key)}
          pinResource={() => onPinResource(key)}
        />
      ))}
      {isProcessing && (
        <LoadingSkeleton count={FETCH_MORE_SIZE} Element={<LoadingCard />} />
      )}
    </CardsContainer>
  );
}

const CardsContainer = styled(Flex)`
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
`;
