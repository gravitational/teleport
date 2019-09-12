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
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { storiesOf } from '@storybook/react'
import { K8s } from './K8s'

storiesOf('Gravity/K8s', module)
  .add('K8s', () => {
    const props = {
      siteId: 'xxx',
      namespaces,
      namespace: 'default',
      k8s,
      category: "pod",
      history: {}
    }

    return (
      <Router history={createMemoryHistory()}>
        <K8s {...props} />
      </Router>
    );
  });


const namespaces = [
  "default",
  "gravity-sbe",
  "kube-public",
  "kube-system",
  "monitoring",
  "tiller"
]

const k8s = {
  "curNamespace": "default",
  "searchValue": ""
}