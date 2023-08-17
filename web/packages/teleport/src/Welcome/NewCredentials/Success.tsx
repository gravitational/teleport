/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { ButtonPrimary, Flex, Image, Text } from 'design';

import { OnboardCard } from 'design/Onboard/OnboardCard';

import { CaptureEvent, userEventService } from 'teleport/services/userEvent';

import { RegisterSuccessProps } from './types';
import shieldCheck from './shield-check.png';

export function RegisterSuccess({
  redirect,
  resetMode = false,
  username = '',
  isDashboard,
}: RegisterSuccessProps) {
  const actionTxt = resetMode ? 'reset' : 'registration';

  const handleRedirect = () => {
    if (username) {
      userEventService.capturePreUserEvent({
        event: CaptureEvent.PreUserCompleteGoToDashboardClickEvent,
        username: username,
      });
    }

    redirect();
  };

  return (
    <OnboardCard center>
      <Text
        typography="h4"
        color="text"
        mb={3}
        style={{ textTransform: 'capitalize' }}
      >
        {actionTxt} successful
      </Text>
      <Flex justifyContent="center" mb={3}>
        <Image src={shieldCheck} width="200px" height="143px" />
      </Flex>
      <Text fontSize={2} color="text.slightlyMuted" mb={4}>
        Congratulations your {actionTxt} is completed.
        <br />
        Proceed to access your account.
      </Text>
      <ButtonPrimary width="100%" size="large" onClick={handleRedirect}>
        Go to {isDashboard ? 'Dashboard' : 'Cluster'}
      </ButtonPrimary>
    </OnboardCard>
  );
}
