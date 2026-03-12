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

import { Link } from 'react-router';

import { ButtonPrimary, ButtonSecondary, Flex, Mark, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';
import { CircleCheck } from 'design/Icon';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import cfg from 'teleport/config';

import { getIntegrationName } from '../getIntegrationName';

export function FinishDialog({
  role,
  gitHubOrgName,
}: {
  role: { created: false } | { created: true; name: string };
  gitHubOrgName: string;
}) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={true}
      onClose={close}
      open={true}
    >
      <DialogHeader css={{ margin: '0 auto' }}>
        <CircleCheck mb={4} size={60} color="success.main" />
      </DialogHeader>
      <DialogContent>
        {!role.created ? (
          <>
            <Text textAlign="center">GitHub integration complete.</Text>
            <Text textAlign="center" mb={1}>
              Once users are logged in with a role that grants them access to
              Git server <Mark>{getIntegrationName(gitHubOrgName)}</Mark>, users
              can use <Mark>tsh</Mark> to perform git commands such as:
            </Text>
          </>
        ) : (
          <Text textAlign="center" mb={1}>
            Successfully created Teleport role <Mark>{role.name}</Mark>. Once
            users login with this role, they can use <Mark>tsh</Mark> to perform
            git commands such as:
          </Text>
        )}
        <TextSelectCopyMulti
          lines={[
            {
              comment: 'To list Git Teleport servers:',
              text: 'tsh git ls',
            },
            {
              comment: 'To clone a new repository using SSH:',
              text: `tsh git clone <git-clone-ssh-url>`,
            },
            {
              comment:
                'To configure an existing git repository,\nchange your current working directory to target repository, then:',
              text: `tsh git config update`,
            },
          ]}
        />
      </DialogContent>
      <DialogFooter css={{ margin: '0 auto' }}>
        <Flex gap="3">
          <ButtonPrimary as={Link} to={cfg.routes.integrations} size="large">
            Go to Integration List
          </ButtonPrimary>
          <ButtonSecondary
            as={Link}
            to={cfg.getIntegrationEnrollRoute(null)}
            size="large"
          >
            Add Another Integration
          </ButtonSecondary>
        </Flex>
      </DialogFooter>
    </Dialog>
  );
}
