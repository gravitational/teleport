/*
Copyright 2019-2022 Gravitational, Inc.

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
import { Box, Text, Link } from 'design';
import { Auth2faType } from 'shared/services';

export default function TwoFAData({ auth2faType, qr }: Props) {
  const imgSrc = `data:image/png;base64,${qr}`;

  if (auth2faType === 'otp') {
    return (
      <Box width="168px">
        <Text typography="paragraph2" mb={3}>
          Scan the QR Code with any authenticator app and enter the generated
          code.
        </Text>
        <img width="152px" src={imgSrc} style={{ border: '8px solid' }} />
        <Text typography="paragraph2" color="text.secondary">
          We recommend{' '}
          <Link href="https://authy.com/download/" target="_blank">
            Authy
          </Link>
          .
        </Text>
      </Box>
    );
  }

  return null;
}

type Props = {
  qr: string;
  auth2faType: Auth2faType;
};
