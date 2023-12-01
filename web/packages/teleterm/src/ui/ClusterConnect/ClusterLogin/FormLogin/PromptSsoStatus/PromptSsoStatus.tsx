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

import React from 'react';
import { Box, ButtonSecondary, Text, Flex } from 'design';

import LinearProgress from 'teleterm/ui/components/LinearProgress';

export default function PromptSsoStatus(props: Props) {
  return (
    <Flex
      flex="1"
      minHeight="40px"
      flexDirection="column"
      justifyContent="space-between"
      alignItems="center"
      p={5}
    >
      <Box mb={4} style={{ position: 'relative' }}>
        <Text bold mb={2} textAlign="center">
          Please follow the steps in the new browser window to authenticate.
        </Text>
        <LinearProgress />
      </Box>
      <ButtonSecondary width={120} size="small" onClick={props.onCancel}>
        Cancel
      </ButtonSecondary>
    </Flex>
  );
}

export type Props = {
  onCancel(): void;
};
