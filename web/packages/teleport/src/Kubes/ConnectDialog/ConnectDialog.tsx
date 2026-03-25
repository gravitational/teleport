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

import { Box, ButtonPrimary, ButtonSecondary, Flex, H3, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { NewTab as NewTabIcon } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';

import cfg from 'teleport/config';
import { generateTshLoginCommand, openNewTab } from 'teleport/lib/util';
import { AuthType } from 'teleport/services/user';

function ConnectDialog(props: Props) {
  const {
    onClose,
    username,
    authType,
    kubeConnectName,
    clusterId,
    accessRequestId,
  } = props;

  const startKubeExecSession = () => {
    const url = cfg.getKubeExecConnectRoute({
      clusterId,
      kubeId: kubeConnectName,
    });

    openNewTab(url);
  };

  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader mb={4}>
        <DialogTitle>
          <Flex gap={2}>
            Connect to:
            <Flex gap={1}>
              <ResourceIcon name="kube" width="24px" height="24px" />
              {kubeConnectName}
            </Flex>
          </Flex>
        </DialogTitle>
      </DialogHeader>
      <DialogContent minHeight="240px" flex="0 0 auto">
        <Box borderBottom={1} mb={4} pb={4}>
          <Text mb={3} bold>
            Open Teleport-authenticated session in the browser:
          </Text>
          <ButtonPrimary size="large" gap={2} onClick={startKubeExecSession}>
            Exec in the browser
            <NewTabIcon />
          </ButtonPrimary>
        </Box>
        <Box mb={4}>
          <H3 mt={1} mb={2}>
            Or connect in the CLI using tsh and kubectl:
          </H3>
          <Text bold as="span">
            Step 1
          </Text>
          {' - Log in to Teleport'}
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
