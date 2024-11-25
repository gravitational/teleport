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

import { Box, ButtonSecondary, Flex, Text } from 'design';

import { LinearProgress } from 'teleterm/ui/components/LinearProgress';

export default function PromptSsoStatus(props: Props) {
  return (
    <Flex gap={4} flexDirection="column" alignItems="flex-start">
      <Box style={{ position: 'relative' }}>
        <Text bold mb={2} textAlign="center">
          Please follow the steps in the new browser window to authenticate.
        </Text>
        <LinearProgress />
      </Box>
      {props.onCancel && (
        <ButtonSecondary onClick={props.onCancel}>Cancel</ButtonSecondary>
      )}
    </Flex>
  );
}

export type Props = {
  onCancel?(): void;
};
