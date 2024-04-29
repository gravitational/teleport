/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React from 'react';
import { ButtonPrimary, Flex, Image, Text } from 'design';

import { OnboardCard } from 'design/Onboard/OnboardCard';

import { CaptureEvent, userEventService } from 'teleport/services/userEvent';
import shieldCheck from 'teleport/assets/shield-check.png';

import { RegisterSuccessProps } from './types';

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
        Congratulations, your {actionTxt} is completed.
        <br />
        Proceed to access your account.
      </Text>
      <ButtonPrimary width="100%" size="large" onClick={handleRedirect}>
        Go to {isDashboard ? 'Dashboard' : 'Cluster'}
      </ButtonPrimary>
    </OnboardCard>
  );
}
