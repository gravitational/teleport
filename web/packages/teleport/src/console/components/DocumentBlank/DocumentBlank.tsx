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
import { Flex, Box, ButtonSecondary } from 'design';
import * as Icons from 'design/Icon';
import { useConsoleContext } from 'teleport/console/consoleContextProvider';
import * as stores from 'teleport/console/stores';
import Document from './../Document';

export default function DocumentBlank(props: PropTypes) {
  const { visible, doc } = props;
  const ctx = useConsoleContext();

  function onClick() {
    ctx.gotoNodeTab(doc.clusterId);
  }

  return (
    <Document visible={visible}>
      <Box mx="auto">
        <Flex flexDirection="column">
          <Icons.Cli fontSize="128px" mt="10" mb="6" color="#0C143D" />
          <ButtonSecondary onClick={onClick} children="New Session" block />
        </Flex>
      </Box>
    </Document>
  );
}

type PropTypes = {
  visible: boolean;
  doc: stores.DocumentBlank;
};
