/*
Copyright 2023 Gravitational, Inc.

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

import React, { useState, useEffect } from 'react';
import QRCode from 'react-qr-code';
import { formatDuration } from 'date-fns';
import { Box, Indicator, Text, ButtonText } from 'design';
import Dialog, {
  DialogHeader,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { ButtonSecondary } from 'design/Button';
import { Danger } from 'design/Alert';

import useAttempt from 'shared/hooks/useAttemptNext';

import auth from 'teleport/services/auth/auth';
import cfg from 'teleport/config';

export function MobileLoginDialog({
  closeDialog,
}: {
  closeDialog: () => void;
}) {
  const [qrContent, setQrContent] = useState('');
  const [timeLeft, setTimeLeft] = useState(300);
  const { run, attempt } = useAttempt();

  let minutesLeft = Math.floor(timeLeft / 60);
  let secondsLeft = timeLeft - minutesLeft * 60;

  function startCountdown(duration: number) {
    setTimeLeft(duration);
    let currTimeLeft = duration;

    let timer = setInterval(function () {
      setTimeLeft(currTimeLeft);
      currTimeLeft--;

      if (currTimeLeft < 0) {
        clearInterval(timer);
      }
    }, 1000);
  }

  useEffect(() => {
    getToken();
  }, []);

  function getToken() {
    run(() =>
      auth.createMobileAuthToken().then(res => {
        setQrContent(
          `go.teleport.mobile://${encodeURIComponent(cfg.baseUrl)}?token=${
            res.token
          }`
        );
        //   setQrContent(`
        //   {
        //   "url": "${cfg.baseUrl}",
        //   "token": "${res.token}"
        //   }
        // `);
        startCountdown(300);
      })
    );
  }

  return (
    <Dialog
      open
      css={`
        display: none;
      `}
    >
      <DialogHeader>
        Scan this code using your iOS device to login to the Teleport app.
      </DialogHeader>
      <DialogContent
        width="100%"
        css={`
          display: flex;
          justify-content: center;
          align-items: center;
        `}
      >
        {attempt.status === 'failed' && (
          <Danger>
            Failed to create mobile auth token: {attempt.statusText}
          </Danger>
        )}
        {attempt.status === 'processing' && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}
        {attempt.status === 'success' && (
          <>
            <QRCode
              css={`
                width: 240px;
                height: 240px;
                border: 6px solid white;
              `}
              value={qrContent}
            />
            <Text color="text.slightlyMuted" mt={2}>
              {timeLeft > 0 ? (
                <>
                  This code will expire in{' '}
                  {formatDuration(
                    { minutes: minutesLeft, seconds: secondsLeft },
                    { format: ['minutes', 'seconds'] }
                  )}
                </>
              ) : (
                <>
                  This code has expired.{' '}
                  <ButtonText onClick={getToken}>Generate a new one</ButtonText>
                </>
              )}
            </Text>
          </>
        )}
      </DialogContent>
      <DialogFooter
        css={`
          display: flex;
          justify-content: flex-end;
        `}
      >
        <ButtonSecondary onClick={closeDialog}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
