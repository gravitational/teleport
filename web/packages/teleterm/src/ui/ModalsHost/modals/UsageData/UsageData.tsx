/**
 * Copyright 2022 Gravitational, Inc.
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
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { ButtonIcon, ButtonPrimary, ButtonSecondary, Link, Text } from 'design';
import { Close } from 'design/Icon';

interface UsageDataProps {
  onCancel(): void;

  onAllow(): void;

  onDecline(): void;
}

export function UsageData(props: UsageDataProps) {
  return (
    <DialogConfirmation
      open={true}
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <form
        onSubmit={e => {
          e.preventDefault();
          props.onAllow();
        }}
      >
        <DialogHeader
          justifyContent="space-between"
          mb={0}
          alignItems="baseline"
        >
          <Text typography="h4" bold>
            Anonymous usage data
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
            Do you agree to Teleport Connect collecting anonymized usage data?
            This will help us improve the product.
          </Text>
          <Text typography="body1" color="text.slightlyMuted">
            To learn more, see{' '}
            <Link
              href="https://goteleport.com/docs/faq/#teleport-connect"
              target="_blank"
            >
              our documentation
            </Link>
            .
          </Text>
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary autoFocus mr={3} type="submit">
            Allow
          </ButtonPrimary>
          <ButtonSecondary type="button" onClick={props.onDecline}>
            Decline
          </ButtonSecondary>
        </DialogFooter>
      </form>
    </DialogConfirmation>
  );
}
