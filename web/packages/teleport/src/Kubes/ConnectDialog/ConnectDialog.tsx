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
import Dialog, {
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogContent,
} from 'design/Dialog';
import { Text, Box, ButtonSecondary } from 'design';

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import { AuthType } from 'teleport/services/user';
import { generateTshLoginCommand } from 'teleport/lib/util';

function ConnectDialog(props: Props) {
  const {
    onClose,
    username,
    authType,
    kubeConnectName,
    clusterId,
    accessRequestId,
  } = props;

  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>connect to kubernetes cluster</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Box mb={4}>
          <Text bold as="span">
            Step 1
          </Text>
          {' - Login to Teleport'}
          <TextSelectCopy
            mt="2"
            text={generateTshLoginCommand({
              authType,
              username,
              clusterId,
              accessRequestId,
            })}
          />
        </Box>
        <Box mb={4}>
          <Text bold as="span">
            Optional
          </Text>{' '}
          - To write kubectl configuration to a separate file instead of having
          your global kubectl configuration modified, run the following command:
          <TextSelectCopy
            mt="2"
            text="export KUBECONFIG=${HOME?}/teleport-kubeconfig.yaml"
          />
        </Box>
        <Box mb={4}>
          <Text bold as="span">
            Step 2
          </Text>
          {' - Select the Kubernetes cluster'}
          <TextSelectCopy mt="2" text={`tsh kube login ${kubeConnectName}`} />
        </Box>
        <Box mb={1}>
          <Text bold as="span">
            Step 3
          </Text>
          {' - Connect to the Kubernetes cluster'}
          <TextSelectCopy mt="2" text={`kubectl get pods`} />
        </Box>
        {accessRequestId && (
          <Box mb={1} mt={3}>
            <Text bold as="span">
              Step 4 (Optional)
            </Text>
            {' - When finished, drop the assumed role'}
            <TextSelectCopy mt="2" text={`tsh request drop`} />
          </Box>
        )}
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  onClose: () => void;
  username: string;
  authType: AuthType;
  kubeConnectName: string;
  clusterId: string;
  accessRequestId?: string;
};

const dialogCss = () => `
  min-height: 400px;
  max-width: 600px;
  width: 100%;
`;

export default ConnectDialog;
