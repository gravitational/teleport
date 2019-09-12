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
import { Certificate } from './Certificate'
import { TlsCert } from 'gravity/cluster/flux/tlscert/store';

storiesOf('Gravity/Certificate', module)
  .add('Certificate', () => (
    <Certificate store={store}/>
  )
);

const store = new TlsCert(
  {
    "issued_to": {
      "cn": "*.gravitational.io",
      "org": null,
      "org_unit": ["Domain Control Validated", "EssentialSSL Wildcard"]
    },
    "issued_by": {
      "cn": "COMODO RSA Domain Validation Secure Server CA",
      "org": ["COMODO CA Limited"],
      "org_unit": null
    },
    "validity": {
      "not_before": "2017-10-13T00:00:00Z",
      "not_after": "2020-12-20T23:59:59Z"
    }
  }
);
