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

const docUrl = 'https://gravitational.com/teleport/docs/';

export default function ButtonAdd(props: Props) {
  if (!props.isEnterprise) {
    return (
      <ButtonPrimary
        {...props}
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

  if (props.canCreate) {
    return (
      <ButtonPrimary width="240px" {...props}>
        Add Application
      </ButtonPrimary>
    );
  }

  return null;
}

type Props = {
  isEnterprise: boolean;
  canCreate: boolean;
  onClick?: () => void;
  mb?: string;
  mx?: string;
  width?: string;
  kind?: string;
};
