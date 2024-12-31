/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import {
  AnimatedProgressBar,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Text,
} from 'design';
import Dialog, { DialogContent } from 'design/DialogConfirmation';
import * as Icons from 'design/Icon';

import { TextIcon } from 'teleport/Discover/Shared';

type EnrollmentDialogProps = {
  clusterName: string;
  status: string;
  error: string;
  close(): void;
  retry(): void;
};

export function EnrollmentDialog({
  status,
  error,
  close,
  retry,
}: EnrollmentDialogProps) {
  function dialogContent() {
    switch (status) {
      case 'enrolling':
        return (
          <>
            <AnimatedProgressBar mb={3} />
            <TextIcon mb={3}>
              <Icons.Clock size="medium" />
              <Text>1. Installing Teleport agent...</Text>
            </TextIcon>
            <Text mb={3}>
              2. Waiting for the Teleport agent to come online (1-5 minutes)
            </Text>
            <ButtonPrimary width="100%" disabled>
              Cancel
            </ButtonPrimary>
          </>
        );

      case 'error':
      case 'alreadyExists':
        return (
          <>
            <Flex mb={5} alignItems="center">
              <Icons.Warning size="large" ml={1} mr={2} color="error.main" />
              <Text>{error}</Text>
            </Flex>
            <Flex gap={4}>
              {status === 'error' && (
                <ButtonPrimary width="50%" onClick={retry}>
                  Retry
                </ButtonPrimary>
              )}
              <ButtonSecondary width="50%" onClick={close}>
                Close
              </ButtonSecondary>
            </Flex>
          </>
        );
    }
  }

  return (
    <Dialog open={true}>
      <DialogContent
        width="460px"
        alignItems="center"
        mb={0}
        textAlign="center"
      >
        <Text bold caps mb={4}>
          EKS Cluster Enrollment
        </Text>
        {dialogContent()}
      </DialogContent>
    </Dialog>
  );
}
