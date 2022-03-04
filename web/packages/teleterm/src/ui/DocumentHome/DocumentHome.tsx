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
import { Text, Box, Flex } from 'design';
import Document from 'teleterm/ui/Document';
import * as types from 'teleterm/ui/services/workspacesService/documentsService';

//TODO: remove

export default function DocumentHome(props: PropTypes) {
  const { visible } = props;
  return (
    <Document visible={visible}>
      <Flex flexDirection="column" alignItems="center" flex="1" width="100%">
        <Box width="100%" maxWidth="60%" mx="auto" textAlign="center" mt="20%">
          <Text mb={2} color="text.secondary" typography="subtitle1">
            Show All Commands <Key>F1</Key>
          </Text>
          <Text mb={2} color="text.secondary" typography="subtitle1">
            Open a new terminal tab <Key mr={1}>Ctrl</Key>+
            <Key mx={1}>Shift</Key>+<Key mx={1}>T</Key>
          </Text>
        </Box>
      </Flex>
    </Document>
  );
}

const Key = props => <Text p={1} as="span" {...props} bg="primary.light" />;

type PropTypes = {
  visible: boolean;
  // doc: types.DocumentHome;
};
