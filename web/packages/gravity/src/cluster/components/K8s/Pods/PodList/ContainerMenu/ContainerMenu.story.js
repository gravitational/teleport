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
import { storiesOf } from '@storybook/react'
import Flex from 'design/Flex';
import ContainerMenu from './ContainerMenu';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';

storiesOf('Gravity/K8s', module)
  .add('ContainerMenu', () => (
    <Router history={createMemoryHistory()}>
      <Flex width="400px" height="100px" alignItems="center" justifyContent="space-around" bg="primary.light">
        <ContainerMenu { ...props}/>
        <SampleMenu isOpen/>
      </Flex>
    </Router>
  ));

class SampleMenu extends React.Component {
  componentDidMount(){
    this.props.isOpen && this.menuRef.onOpen();
  }

  render(){
    return (
      <ContainerMenu ref={e => this.menuRef = e } { ...props} />
    )
  }
}


const props = {
  logsEnabled: true,
  container: {
    name: 'Mivwasut',
    logUrl: 'Sarfiwiw',
    pod: 'Tidinuv',
    serverId: 'Jotekda',
    namespace: 'Downiuwi',
    sshLogins: [
      "root",
      "jazrafiba",
      "evubale",
      "ipizodu",
    ]
  }
}