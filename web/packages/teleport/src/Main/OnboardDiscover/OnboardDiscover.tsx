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

import { ButtonPrimary, ButtonText, H2, Image, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';

import { userEventService } from 'teleport/services/userEvent';
import { CaptureEvent } from 'teleport/services/userEvent/types';

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
        <H2>Start by adding your first resource</H2>
        <Text mt={3}>
          Teleport allows users to access a wide variety of resources, from
          Linux servers to Kubernetes clusters.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary width="100%" size="large" onClick={handleOnboard}>
          Add my first resource
        </ButtonPrimary>
        <ButtonText mt={2} width="100%" size="large" onClick={handleClose}>
          I'll do that later
        </ButtonText>
      </DialogFooter>
    </Dialog>
  );
}
