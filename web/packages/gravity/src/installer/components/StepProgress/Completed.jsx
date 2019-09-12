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
import cfg from 'gravity/config';
import { CircleCheck } from 'design/Icon/Icon';
import { Text, Card, Box, ButtonPrimary } from 'design';

export default function Completed(props){
  const completeInstallUrl = cfg.getInstallerLastStepUrl(props.siteId);
  return (
    <StyledCompleted flexDirection="column" bg="light" py="5" px="10" color="text.onLight"  {...props}>
      <CircleCheck mb="5" color="success" fontSize="100px" />
      <Box as={Text} typography="h5" mb="8">
        The application has been installed successfully. Please continue and configure your
        application to finish the setup process.
      </Box>
      <ButtonPrimary as="a" href={completeInstallUrl} size="large">
        { `Continue & finish setup` }
      </ButtonPrimary>
    </StyledCompleted>
  );
}


const StyledCompleted = styled(Card)`
  text-align: center;

`