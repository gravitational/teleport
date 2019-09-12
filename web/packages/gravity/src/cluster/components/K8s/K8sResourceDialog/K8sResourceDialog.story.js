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
import K8sResourceDialog from './K8sResourceDialog'

const defaultProps = {
  readOnly: true,
  name: "resource name",
  namespace: "namespace name",
  onClose(){ },
}

storiesOf('Gravity/K8s', module)
  .add('K8sResourceDialog-View', () => {
    const props = {
      ...defaultProps,
      readOnly: true,
      resource
    }

    return (
      <K8sResourceDialog {...props} />
    );
  })
  .add('K8sResourceDialog-Edit', () => {
    const props = {
      ...defaultProps,
      readOnly: false,
      resource
    }

    return (
      <K8sResourceDialog {...props} />
    );
  });

const resource = {
  "metadata": {
    "name": "default-token-jwx5x",
    "namespace": "kube-public",
    "selfLink": "/api/v1/namespaces/default/secrets/default-token-jwx5x",
    "uid": "39bcabf3-75bc-11e9-ae57-0ebb30a2cfe6",
    "resourceVersion": "289",
    "creationTimestamp": "2019-05-13T20:18:09Z",
    "annotations": {
      "kubernetes.io/service-account.name": "default",
      "kubernetes.io/service-account.uid": "3988fc22-75bc-11e9-ae57-0ebb30a2cfe6"
    },
    "managedFields": [
      {
        "manager": "kube-controller-manager",
        "operation": "Update",
        "apiVersion": "v1",
        "time": "2019-05-13T20:18:09Z",
        "fields": { "f:data": { ".": null, "f:ca.crt": null, "f:namespace": null, "f:token": null }, "f:metadata": { "f:annotations": { ".": null, "f:kubernetes.io/service-account.name": null, "f:kubernetes.io/service-account.uid": null } }, "f:type": null }
      }
    ]
  },
  "data": {
    "ca.crt": "LS0tLS1CRUdJTiBDRVJUS",
    "namespace": "ZGVmYXVsdA==",
    "token": "ZXlKaGJHY2lPaUpTVXpJMU5p"
  }
}
