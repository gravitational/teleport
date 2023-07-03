/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { ButtonIcon, ButtonSecondary, Text } from 'design';
import { Close } from 'design/Icon';

import { ResourceSearchError } from 'teleterm/ui/services/resources';

import type * as uri from 'teleterm/ui/uri';

export function ResourceSearchErrors(props: {
  errors: ResourceSearchError[];
  getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string;
  onCancel: () => void;
}) {
  const formattedErrorText = props.errors
    .map(error => error.messageAndCauseWithClusterName(props.getClusterName))
    .join('\n\n');

  return (
    <DialogConfirmation
      open={true}
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '800px',
        width: '100%',
      })}
    >
      <DialogHeader justifyContent="space-between" mb={0} alignItems="baseline">
        <Text typography="h4" bold>
          Resource search errors
        </Text>
        <ButtonIcon
          type="button"
          onClick={props.onCancel}
          color="text.slightlyMuted"
        >
          <Close fontSize={5} />
        </ButtonIcon>
      </DialogHeader>
      <DialogContent mb={4}>
        <Text typography="body1" color="text.slightlyMuted">
          <pre
            css={`
              padding: ${props => props.theme.space[2]}px;
              background-color: ${props => props.theme.colors.levels.sunken};
              color: ${props => props.theme.colors.text.main};

              white-space: pre-wrap;
              max-height: calc(${props => props.theme.space[6]}px * 10);
              overflow-y: auto;
            `}
          >
            {formattedErrorText}
          </pre>
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary type="button" onClick={props.onCancel}>
          Close
        </ButtonSecondary>
      </DialogFooter>
    </DialogConfirmation>
  );
}
