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
import styled from 'styled-components';
import { Warning } from 'design/Icon/Icon';
import { Text, Card, Box, ButtonWarning } from 'design';
import { makeDownloadable } from 'gravity/services/downloader';

export default function Failed({tarballUrl, ...styles}){

  function onClick () {
    location.href= makeDownloadable(tarballUrl);
  }

  return (
    <StyledCompleted flexDirection="column" bg="light" py="5" px="10" color="text.onLight" {...styles}>
      <Warning mb="5" color="error.main" fontSize="100px" />
      <Box as={Text} typography="h5" mb="8">
        Something went wrong with the install. We've attached a tarball which has diagnostic logs that our team will need to review. We sincerely apologize for any inconvenience
      </Box>
      <ButtonWarning size="large" onClick={onClick}>
        Download tarball
      </ButtonWarning>
    </StyledCompleted>
  );
}

const StyledCompleted = styled(Card)`
  text-align: center;
`