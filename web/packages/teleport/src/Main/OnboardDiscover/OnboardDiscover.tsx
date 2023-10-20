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
import { ButtonPrimary, ButtonText, Image, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';

import { CaptureEvent } from 'teleport/services/userEvent/types';
import { userEventService } from 'teleport/services/userEvent';

import resourcesPng from './resources.png';

export function OnboardDiscover({
  onClose,
  onOnboard,
}: {
  onClose(): void;
  onOnboard(): void;
}) {
  const handleOnboard = () => {
    userEventService.captureUserEvent({
      event: CaptureEvent.OnboardAddFirstResourceClickEvent,
    });
    onOnboard();
  };

  const handleClose = () => {
    userEventService.captureUserEvent({
      event: CaptureEvent.OnboardAddFirstResourceLaterClickEvent,
    });
    onClose();
  };

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '450px',
        width: '100%',
        overflow: 'initial',
      })}
      // disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader mx="auto">
        <Image src={resourcesPng} width="350px" height="218.97px" />
      </DialogHeader>
      <DialogContent textAlign="center">
        <Text bold typography="h4">
          Start by adding your first resource
        </Text>
        <Text mt={3}>
          Teleport allows users to access a wide variety of resources, from
          Linux servers to Kubernetes clusters.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary width="100%" size="large" onClick={handleOnboard}>
          add my first resource
        </ButtonPrimary>
        <ButtonText pt={2} width="100%" size="large" onClick={handleClose}>
          I'll do that later
        </ButtonText>
      </DialogFooter>
    </Dialog>
  );
}
