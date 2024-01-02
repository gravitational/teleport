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

import { Box, ButtonSecondary, Text, Flex } from 'design';

import LinearProgress from 'teleterm/ui/components/LinearProgress';

export default function PromptSsoStatus(props: Props) {
  return (
    <Flex p={4} gap={4} flexDirection="column" alignItems="flex-start">
      <Box style={{ position: 'relative' }}>
        <Text bold mb={2} textAlign="center">
          Please follow the steps in the new browser window to authenticate.
        </Text>
        <LinearProgress />
      </Box>
      <ButtonSecondary onClick={props.onCancel}>Cancel</ButtonSecondary>
    </Flex>
  );
}

export type Props = {
  onCancel(): void;
};
