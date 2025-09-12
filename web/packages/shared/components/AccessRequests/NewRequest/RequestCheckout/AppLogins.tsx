/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Box, Flex, Text } from 'design';
import { CrossIcon } from 'shared/components/AccessRequests/NewRequest/RequestCheckout/CrossIcon';
import { Attempt } from 'shared/hooks/useAttemptNext';

export function ItemLoginSelect<T>({
  item,
  toggleLogin,
  clearAttempt,
  createAttempt,
}: {
  item: T & { logins: { name: string; id: string }[] };
  toggleLogin: (id: string) => (item: T) => void;
  clearAttempt: () => void;
  createAttempt: Attempt;
}) {
  return (
    <Flex flexDirection="column" gap={2} mt={2} ml={2}>
      <Text typography="body3">Logins</Text>
      {item.logins.map(login => (
        <Flex
          key={login.id}
          gap={5}
          alignItems="center"
          justifyContent="space-between"
        >
          <Box>{login.name}</Box>
          <CrossIcon
            item={item}
            toggleResource={toggleLogin(login.id)}
            clearAttempt={clearAttempt}
            createAttempt={createAttempt}
          />
        </Flex>
      ))}
    </Flex>
  );
}
