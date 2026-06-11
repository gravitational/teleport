/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { useState } from 'react';

import { Box, ButtonPrimary, ButtonSecondary, Card, Flex, Text } from 'design';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import { ResourceListItem } from 'shared/components/UnifiedResources/ListView/ResourceListItem';
import {
  PinningSupport,
  UnifiedResourceDatabase,
} from 'shared/components/UnifiedResources/types';
import { makeUnifiedResourceViewItemDatabase } from 'shared/components/UnifiedResources/shared/viewItemsFactory';

import { LogoHero } from 'teleport/components/LogoHero';

const databaseResource: UnifiedResourceDatabase = {
  kind: 'db',
  name: 'production-db',
  description: 'MySQL database',
  type: '',
  protocol: 'mysql',
  labels: [],
};

const databaseViewItem = makeUnifiedResourceViewItemDatabase(databaseResource, {
  ActionButton: <span />,
});
databaseViewItem.SecondaryIcon = () => null;

export function DelegationAuthorize() {
  const [allowAdditionalAccess, setAllowAdditionalAccess] = useState(true);

  return (
    <>
      <LogoHero />
      <Card my="5" mx="auto" maxWidth={500} minWidth={300} py={4} px={4}>
        <Text typography="h1" mb={3} textAlign="center">
          Delegate access
        </Text>
        <Text color="text.slightlyMuted" mb={4} textAlign="center">
          An application would like to access the following resources on your
          behalf:
        </Text>
        <Box
          border={1}
          borderColor="spotBackground.0"
          borderRadius={2}
          mb={4}
          overflow="hidden"
        >
          <ResourceListItem
            pinned={false}
            pinResource={() => { }}
            selectResource={() => { }}
            selected={false}
            pinningSupport={PinningSupport.Hidden}
            expandAllLabels={false}
            onShowStatusInfo={() => { }}
            showingStatusInfo={false}
            viewItem={databaseViewItem}
            visibleInputFields={{
              checkbox: false,
              pin: false,
              copy: false,
              hoverState: false,
            }}
          />
        </Box>
        <FieldCheckbox
          mb={4}
          checked={allowAdditionalAccess}
          onChange={event => setAllowAdditionalAccess(event.target.checked)}
          label="Allow the application to request additional access on your behalf"
          size="small"
        />
        <Flex gap={3}>
          <ButtonSecondary block>Reject</ButtonSecondary>
          <ButtonPrimary block>Allow</ButtonPrimary>
        </Flex>
      </Card>
    </>
  );
}
