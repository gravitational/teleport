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

import { Flex } from 'design';

import { LoadingSkeleton } from '../shared/LoadingSkeleton';
import { ResourceViewProps } from '../types';
import { FETCH_MORE_SIZE } from '../UnifiedResources';
import { LoadingListItem } from './LoadingListItem';
import { ResourceListItem } from './ResourceListItem';

export function ListView({
  mappedResources,
  onLabelClick,
  pinnedResources,
  selectedResources,
  onSelectResource,
  onPinResource,
  pinningSupport,
  isProcessing,
  expandAllLabels,
}: ResourceViewProps) {
  return (
    <Flex className="ListContainer">
      {mappedResources.map(({ item, key }) => (
        <ResourceListItem
          key={key}
          name={item.name}
          ActionButton={item.ActionButton}
          primaryIconName={item.primaryIconName}
          onLabelClick={onLabelClick}
          SecondaryIcon={item.SecondaryIcon}
          listViewProps={item.listViewProps}
          labels={item.labels}
          pinned={pinnedResources.includes(key)}
          pinningSupport={pinningSupport}
          requiresRequest={item.requiresRequest}
          selected={selectedResources.includes(key)}
          selectResource={() => onSelectResource(key)}
          pinResource={() => onPinResource(key)}
          expandAllLabels={expandAllLabels}
        />
      ))}
      {isProcessing && (
        <LoadingSkeleton
          count={FETCH_MORE_SIZE}
          Element={<LoadingListItem />}
        />
      )}
    </Flex>
  );
}
