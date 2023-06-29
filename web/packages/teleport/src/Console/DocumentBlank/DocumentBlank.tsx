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
import { Flex, ButtonPrimary } from 'design';
import * as Icons from 'design/Icon';

import { useConsoleContext } from 'teleport/Console/consoleContextProvider';
import * as stores from 'teleport/Console/stores';

import Document from './../Document';

export default function DocumentBlank(props: PropTypes) {
  const { visible, doc } = props;
  const ctx = useConsoleContext();

  function onClick() {
    ctx.gotoNodeTab(doc.clusterId);
  }

  return (
    <Document visible={visible}>
      <Flex flexDirection="column" alignItems="center" flex="1">
        <Icons.Cli
          fontSize="256px"
          mt="10"
          mb="6"
          css={`
            color: ${props => props.theme.colors.spotBackground[1]};
          `}
        />
        <ButtonPrimary onClick={onClick} children="Start a New Session" />
      </Flex>
    </Document>
  );
}

type PropTypes = {
  visible: boolean;
  doc: stores.DocumentBlank;
};
