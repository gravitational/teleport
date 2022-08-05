/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
