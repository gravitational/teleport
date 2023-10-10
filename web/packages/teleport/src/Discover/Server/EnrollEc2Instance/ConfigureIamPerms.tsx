/**
 * Copyright 2023 Gravitational, Inc.
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
import { Link, Text } from 'design';

import { CommandBox } from 'teleport/Discover/Shared/CommandBox';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

export function ConfigureIamPerms({ scriptUrl }: { scriptUrl: string }) {
  return (
    <CommandBox
      header={
        <>
          <Text bold>Configure your AWS IAM permissions</Text>
          <Text typography="subtitle1" mb={3}>
            We were unable to list your EC2 instances. Run the command below on
            your{' '}
            <Link
              href="https://console.aws.amazon.com/cloudshell/home"
              target="_blank"
            >
              AWS CloudShell
            </Link>{' '}
            to configure your IAM permissions. Then press the refresh button
            above.
          </Text>
        </>
      }
      hasTtl={false}
    >
      <TextSelectCopyMulti
        lines={[{ text: `bash -c "$(curl '${scriptUrl}')"` }]}
      />
    </CommandBox>
  );
}
