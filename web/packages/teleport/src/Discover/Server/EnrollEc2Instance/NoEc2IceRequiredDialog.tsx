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

import React from 'react';
import { Text, Flex, ButtonPrimary } from 'design';
import * as Icons from 'design/Icon';
import Dialog, { DialogContent } from 'design/DialogConfirmation';

import { Mark } from 'teleport/Discover/Shared';
import { NodeMeta, useDiscover } from 'teleport/Discover/useDiscover';

export function NoEc2IceRequiredDialog({ nextStep }: { nextStep: () => void }) {
  const { agentMeta } = useDiscover();
  const typedAgentMeta = agentMeta as NodeMeta;

  return (
    <Dialog disableEscapeKeyDown={false} open={true}>
      <DialogContent
        width="460px"
        alignItems="center"
        mb={0}
        textAlign="center"
      >
        <Flex mb={5}>
          <Icons.Check size="small" ml={1} mr={2} color="success.main" />
          <Text>
            The discovery service can take a few minutes to finish
            auto-enrolling resources found in region{' '}
            <Mark>{typedAgentMeta.awsRegion}</Mark>.
          </Text>
        </Flex>
        <ButtonPrimary width="100%" onClick={() => nextStep()}>
          Next
        </ButtonPrimary>
      </DialogContent>
    </Dialog>
  );
}
