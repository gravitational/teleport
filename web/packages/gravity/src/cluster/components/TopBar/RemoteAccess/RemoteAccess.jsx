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
import { Flex, Text } from 'design';
import Switch from './Switch';
import RemoteAccessDialog from './RemoteAccesssDialog';
import { RemoteAccessEnum } from 'gravity/services/enums';

export default function RemoteAssistance(props) {
  const [ isOpen, setIsOpen ] = React.useState(false);
  const { onChange, remoteAccess } = props;

  // do not display if remote access is not configured
  if(remoteAccess === RemoteAccessEnum.NA){
    return null;
  }

  const isEnabled = remoteAccess === RemoteAccessEnum.ON;

  function onConfirmed(){
    return onChange(!isEnabled);
  }

  return (
    <Flex alignItems="center" mr="4">
      <Text mr="2" typography="subtitle2" color="text.primary">
        REMOTE ASSITANCE
      </Text>
      <Switch checked={isEnabled} onChange={ () => setIsOpen(true) } />
      { isOpen && <RemoteAccessDialog enabled={isEnabled} onConfirmed={onConfirmed} onClose={ () => setIsOpen(false) } /> }
    </Flex>
  )
}

