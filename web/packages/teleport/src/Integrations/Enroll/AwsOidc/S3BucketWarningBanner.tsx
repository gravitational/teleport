/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { ButtonText, Box, Text, Flex, ButtonBorder } from 'design';
import { OutlineWarn } from 'design/Alert/Alert';
import { Notification } from 'design/Icon';
import styled from 'styled-components';

export const S3BucketWarningBanner = ({
  onClose,
  onContinue,
  reviewing = false,
  btnFlexWrap = false,
}: {
  onClose(): void;
  onContinue(): void;
  reviewing?: boolean;
  btnFlexWrap?: boolean;
}) => {
  return (
    <OutlineWarn css={{ justifyContent: 'normal', margin: 0 }}>
      <Box>
        <BellIcon size={18} />
      </Box>
      <Flex gap={2}>
        <Box>
          <Text mb={2}>
            It is recommended to use an S3 bucket to host the public keys.
          </Text>
          <Text>
            Without an S3 bucket, you will be required to append the new
            certificate's thumbprint in the AWS IAM/Identity Provider section
            after you have renewed and started using the new certificate.
          </Text>
        </Box>
        <Flex
          gap={3}
          alignItems="center"
          css={
            btnFlexWrap ? { justifyContent: 'center', flexWrap: 'wrap' } : ``
          }
        >
          {reviewing ? (
            <ButtonBorder onClick={onClose} width="80px">
              Ok
            </ButtonBorder>
          ) : (
            <>
              <ButtonBorder onClick={onContinue} width="130px">
                Continue
              </ButtonBorder>
              <ButtonText onClick={onClose} width="100px">
                Cancel
              </ButtonText>
            </>
          )}
        </Flex>
      </Flex>
    </OutlineWarn>
  );
};

const BellIcon = styled(Notification)`
  background-color: ${p => p.theme.colors.warning.hover};
  border-radius: 100px;
  height: 32px;
  width: 32px;
  color: ${p => p.theme.colors.text.primaryInverse};
  margin-right: ${p => p.theme.space[3]}px;
`;
