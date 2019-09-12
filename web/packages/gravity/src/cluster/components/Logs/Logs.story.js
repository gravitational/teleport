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
import styled from 'styled-components';
import { storiesOf } from '@storybook/react'
import { ClusterLogs } from './Logs'

storiesOf('Gravity/Logs', module)
  .add('Logs with error', () => {
    return (
      <Container>
        <ClusterLogs {...props} />
      </Container>
    );
  })

const props = {
  query: '',
  logForwarderStore: null,
  isSettingsOpen: false,
  refreshCount: 0,
  attempt: { isFailed: true, message: 'unable to load log forwarders'},
  onSearch: () => null,
  onRefresh: () => null,
  onOpenSettings: () => null,
  onCloseSettings: () => null,
}


const Container = styled.div`
  position: fixed;
  width: 100%;
  height: 100%;
`