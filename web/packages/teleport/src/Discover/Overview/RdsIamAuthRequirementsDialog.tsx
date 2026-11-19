/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { H3, Link, Text } from 'design';
import { ButtonSecondary } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import {
  AwsRdsConnectInfo,
  AwsRdsDatabaseUserInfo,
} from 'teleport/Discover/Database/SetupAccess/AwsRdsAuthRequirements';

export function RdsIamAuthRequirementsDialog({ onClose }: { onClose(): void }) {
  return (
    <Dialog
      dialogCss={dialogCss}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>RDS IAM Authentication Requirements</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Text mb={3}>
          Teleport uses AWS IAM authentication to connect to RDS databases. Two
          things need to be true:
        </Text>

        <H3 mb={2}>1. Enable IAM authentication</H3>
        <Text mb={4}>
          Databases without it show as issues in the integration's health.
          Follow the AWS{' '}
          <Link
            target="_blank"
            href="https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.Enabling.html"
          >
            documentation
          </Link>{' '}
          to enable it.
        </Text>

        <H3 mb={2}>2. Set up database users to log in with IAM</H3>
        <AwsRdsDatabaseUserInfo />
        <AwsRdsConnectInfo />
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

const dialogCss = () => `
  max-width: 650px;
  width: 100%;
`;
