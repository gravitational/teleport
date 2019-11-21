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
import { Text, Box } from 'design';
import FieldInputSsh from './FieldInputSsh';

export default function Home({ onNew, visible, clusterId }) {
  const inputRef = React.useRef();

  React.useEffect(() => {
    if (!inputRef.current) {
      return;
    }

    if (visible === true) {
      inputRef.current.focus();
    }
  }, [visible]);

  return (
    <Box flexDirection="column" height="100%" width="100%">
      <Box mt={10}>
        <Text typography="h4" mb="2" textAlign="center">
          Cluster {clusterId}
        </Text>
      </Box>
      <FieldInputSsh ref={inputRef} mx="auto" width="80%" onEnter={onNew} />
    </Box>
  );
}
