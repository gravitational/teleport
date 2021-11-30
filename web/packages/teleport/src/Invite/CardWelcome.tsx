/*
Copyright 2021 Gravitational, Inc.

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
import { Card, Box, Text, ButtonPrimary } from 'design';
import cfg from 'teleport/config';
import history from 'teleport/services/history';

export default function CardWelcome({ tokenId, inviteMode = true }: Props) {
  const title = inviteMode ? 'Welcome to Teleport' : 'Reset Password';
  const description = inviteMode
    ? 'Please click the button below to create an account'
    : 'Please click the button below to begin recovery of your account';
  const buttonText = inviteMode ? 'Get started' : 'Continue';

  const nextStepPath = inviteMode
    ? cfg.getUserInviteTokenContinueRoute(tokenId)
    : cfg.getUserResetTokenContinueRoute(tokenId);

  return (
    <Card bg="primary.light" my={6} mx="auto" width="464px">
      <Box p={6}>
        <Text typography="h2" mb={3} textAlign="center" color="light">
          {title}
        </Text>
        <Text typography="h5" mb={3} textAlign="center">
          {description}
        </Text>
        <ButtonPrimary
          width="100%"
          mt={3}
          size="large"
          onClick={() => history.push(nextStepPath)}
        >
          {buttonText}
        </ButtonPrimary>
      </Box>
    </Card>
  );
}

type Props = {
  tokenId: string;
  inviteMode?: boolean;
};
