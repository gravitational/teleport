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

import React from 'react'
import htmlUtils from 'gravity/lib/htmlUtils';
import Dialog, { DialogFooter, DialogTitle, DialogContent, DialogHeader } from 'design/DialogConfirmation';
import { Text, Box, Flex, LabelInput, ButtonSecondary, ButtonPrimary } from 'design';
import CmdText from 'gravity/components/CmdText';


export default function ClusterInfoDialog(props){
  const { onClose, cmd, publicUrls, internalUrls } = props;

  const $publicUrls = publicUrls.map(url => (
    <BoxUrl key={url} url={url} />
  ))

  const $internalUrls = internalUrls.map(url => (
    <BoxUrl key={url} url={url} />
  ))

  return (
    <Dialog
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <Box width="600px">
        <DialogHeader>
          <DialogTitle>
            Cluster Information
          </DialogTitle>
        </DialogHeader>
        <DialogContent>
          <LabelInput>
            Public URL
          </LabelInput>
          {$publicUrls}
          <LabelInput>
            Internal URL
          </LabelInput>
          {$internalUrls}
          <LabelInput>
            Login to this cluster from console
          </LabelInput>
          <CmdText cmd={cmd} />
        </DialogContent>
        <DialogFooter>
          <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
        </DialogFooter>
      </Box>
    </Dialog>
  );
}

const BoxUrl = ({url}) => {
  const ref = React.useRef();
  function onCopyClick(){
    htmlUtils.copyToClipboard(url);
    htmlUtils.selectElementContent(ref.current);
  }

  return (
    <Flex bg="primary.light" p="2" mb="3" alignItems="center" justifyContent="space-between">
      <Text ref={ref} style={{ wordBreak: "break-all" }} mr="3">{url}</Text>
      <ButtonPrimary onClick={onCopyClick} size="small">Copy</ButtonPrimary>
    </Flex>
  )
}