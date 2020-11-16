/*
Copyright 2020 Gravitational, Inc.

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
import { ButtonPrimary } from 'design';

const docUrl =
  'https://gravitational.com/teleport/docs/admin-guide/#adding-nodes-to-the-cluster';

export default function ButtonAdd(props: Props) {
  const { isEnterprise, canCreate, isLeafCluster, ...rest } = props;

  if (!isEnterprise) {
    return (
      <ButtonPrimary
        {...rest}
        width="240px"
        onClick={() => null}
        as="a"
        target="_blank"
        href={docUrl}
      >
        View documentation
      </ButtonPrimary>
    );
  }

  if (canCreate) {
    const title = isLeafCluster
      ? 'Adding a server to a leaf cluster is not supported'
      : 'Add a server to the root cluster';

    return (
      <ButtonPrimary
        title={title}
        disabled={isLeafCluster}
        width="240px"
        {...props}
      >
        Add Server
      </ButtonPrimary>
    );
  }

  return null;
}

type Props = {
  isLeafCluster: boolean;
  isEnterprise: boolean;
  canCreate: boolean;
  onClick?: () => void;
  mb?: string;
  mx?: string;
  width?: string;
  kind?: string;
};
