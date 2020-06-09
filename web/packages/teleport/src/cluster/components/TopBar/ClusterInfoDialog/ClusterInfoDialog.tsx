/*
Copyright 2019 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import copyToClipboard from 'design/utils/copyToClipboard';
import selectElementContent from 'design/utils/selectElementContent';
import { Text, Box, Flex, LabelInput, ButtonPrimary } from 'design';
import Dialog, {
  DialogFooter,
  DialogTitle,
  DialogContent,
  DialogHeader,
} from 'design/DialogConfirmation';

type ClusterInfoDialogProps = {
  onClose: () => void;
  clusterId: string;
  publicURL: string;
  authVersion: string;
  proxyVersion: string;
};

const ClusterInfoDialog: React.FC<ClusterInfoDialogProps> = ({
  onClose,
  clusterId,
  publicURL,
  authVersion,
  proxyVersion,
}) => {
  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <Box width="600px">
        <DialogHeader>
          <DialogTitle>Cluster Information</DialogTitle>
        </DialogHeader>
        <DialogContent>
          <LabelInput>Public URL</LabelInput>
          <PublicURL url={publicURL} />
          <Attribute title="Cluster Name" value={clusterId} />
          <Attribute title="Auth Service Version" value={authVersion} />
          <Attribute title="Proxy Service Version" value={proxyVersion} />
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary onClick={onClose}>Done</ButtonPrimary>
        </DialogFooter>
      </Box>
    </Dialog>
  );
};

const Attribute = ({ title = '', value = null }) => (
  <Flex mb={3}>
    <Text typography="body2" bold mr={3}>
      {title}:
    </Text>
    <Text typography="body2">{value}</Text>
  </Flex>
);

const PublicURL = ({ url = '' }) => {
  const ref = React.useRef();
  const [copyCmd, setCopyCmd] = React.useState(() => 'Copy');

  function onCopyClick() {
    copyToClipboard(url).then(() => setCopyCmd('Copied'));
    selectElementContent(ref.current);
  }

  return (
    <Flex
      bg="primary.light"
      p="2"
      mb="5"
      alignItems="center"
      justifyContent="space-between"
    >
      <Text ref={ref} style={{ wordBreak: 'break-all' }} mr="3">
        {url}
      </Text>
      <ButtonPrimary onClick={onCopyClick} size="small">
        {copyCmd}
      </ButtonPrimary>
    </Flex>
  );
};

export default ClusterInfoDialog;
