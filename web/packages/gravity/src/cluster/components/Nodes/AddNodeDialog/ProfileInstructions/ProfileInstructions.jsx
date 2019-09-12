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
import { LabelInput, Text, ButtonSecondary } from 'design';
import { DialogContent, DialogFooter} from 'design/DialogConfirmation';
import CmdText from 'gravity/components/CmdText';

export default function ProfileInstructions(props){
  const { joinCmd, downloadCmd, onClose } = props;
  return (
    <React.Fragment>
      <DialogContent minHeight="200px">
        <Text typography="h6" mb="3" caps color="primary.contrastText">
          NEXT ADD EXISTING NODE
        </Text>
        <LabelInput>
          Step 1: Run this command to download gravity binaries
        </LabelInput>
        <CmdText mb="4" cmd={downloadCmd}/>
        <LabelInput>
         Step 2: Run this command to join the cluster
        </LabelInput>
        <CmdText cmd={joinCmd}/>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>
          DONE
        </ButtonSecondary>
      </DialogFooter>
    </React.Fragment>
  )
}