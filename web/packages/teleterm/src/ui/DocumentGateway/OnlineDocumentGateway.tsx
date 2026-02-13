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

import { PropsWithChildren, useMemo, useRef, useState } from 'react';
import styled from 'styled-components';

import {
  Alert,
  Box,
  Button,
  ButtonSecondary,
  Flex,
  H1,
  H2,
  Link,
  Stack,
  Text,
} from 'design';
import * as Alerts from 'design/Alert';
import { ChevronDown, ChevronRight } from 'design/Icon';
import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import { FieldSelect } from 'shared/components/FieldSelect';
import Validation from 'shared/components/Validation';
import { Attempt, RunFuncReturnValue } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import { CliCommand } from '../components/CliCommand';
import { ConfigFieldInput, PortFieldInput } from '../components/FieldInputs';

export function OnlineDocumentGateway(props: {
  changeDbName: (name: string) => RunFuncReturnValue<void>;
  changeDbNameAttempt: Attempt<void>;
  changePort: (port: string) => RunFuncReturnValue<void>;
  changePortAttempt: Attempt<void>;
  disconnect: () => RunFuncReturnValue<void>;
  disconnectAttempt: Attempt<void>;
  gateway: Gateway;
  runCliCommand: () => void;
  autoUsersEnabled?: boolean;
}) {
  const { gateway, autoUsersEnabled } = props;
  const [isAdvancedOpen, setIsAdvancedOpen] = useState(false);

  const isPortOrDbNameProcessing =
    props.changeDbNameAttempt.status === 'processing' ||
    props.changePortAttempt.status === 'processing';
  const hasError =
    props.changeDbNameAttempt.status === 'error' ||
    props.changePortAttempt.status === 'error';
  const formRef = useRef<HTMLFormElement>(null);

  const handleChangeDbName = useMemo(() => {
    return debounce((value: string) => {
      props.changeDbName(value);
    }, 150);
  }, [props.changeDbName]);

  const handleChangePort = useMemo(() => {
    return debounce((value: string) => {
      if (formRef.current.reportValidity()) {
        props.changePort(value);
      }
    }, 1000);
  }, [props.changePort]);

  const $errors = hasError && (
    <Flex flexDirection="column" gap={2} mb={3}>
      {props.changeDbNameAttempt.status === 'error' && (
        <Alerts.Danger mb={0} details={props.changeDbNameAttempt.statusText}>
          Could not change the database name
        </Alerts.Danger>
      )}
      {props.changePortAttempt.status === 'error' && (
        <Alerts.Danger mb={0} details={props.changePortAttempt.statusText}>
          Could not change the port number
        </Alerts.Danger>
      )}
    </Flex>
  );

  return (
    <Box maxWidth="680px" width="100%" mx="auto" mt="4" px="5">
      <Flex justifyContent="space-between" mb="4" flexWrap="wrap" gap={2}>
        <H1>Database Connection</H1>
        <ButtonSecondary size="small" onClick={props.disconnect}>
          Close Connection
        </ButtonSecondary>
      </Flex>

      {props.disconnectAttempt.status === 'error' && (
        <Alert details={props.disconnectAttempt.statusText}>
          Could not close the connection
        </Alert>
      )}

      <H2 mb={2}>Connect with CLI</H2>
      <Stack gap={2} alignItems="normal">
        <Flex as="form" ref={formRef}>
          <Validation>
            <PortFieldInput
              label="Port"
              defaultValue={gateway.localPort}
              onChange={e => handleChangePort(e.target.value)}
              mb={0}
            />
            <ConfigFieldInput
              label="Database Name"
              defaultValue={gateway.targetSubresourceName}
              onChange={e => handleChangeDbName(e.target.value)}
              spellCheck={false}
              ml={2}
              mb={0}
            />
            {autoUsersEnabled && (
              <ConfigFieldInput
                label="User"
                value={gateway.targetUser}
                toolTipContent="Using auto provisioned user, you cannot change the database user."
                readonly
                disabled
                ml={2}
                mb={0}
              />
            )}
          </Validation>
        </Flex>
        {gateway?.databaseRoles?.length > 0 && (
          <AdvancedRoles gateway={gateway} isAdvancedOpen={isAdvancedOpen} setIsAdvancedOpen={setIsAdvancedOpen} />
        )}
        <CliCommand
          cliCommand={props.gateway.gatewayCliCommand.preview}
          isLoading={isPortOrDbNameProcessing}
          button={{ onClick: props.runCliCommand }}
        />
        {$errors}
      </Stack>

      <H2 mt={3} mb={2}>
        Connect with GUI
      </H2>
      <Text
        // Break long usernames rather than putting an ellipsis.
        css={`
          word-break: break-word;
        `}
      >
        Configure the GUI database client to connect to host{' '}
        <code>{gateway.localAddress}</code> on port{' '}
        <code>{gateway.localPort}</code> as user{' '}
        <code>{gateway.targetUser}</code>.
      </Text>
      <Text>
        The connection is made through an authenticated proxy so no extra
        credentials are necessary. See{' '}
        <Link
          href="https://goteleport.com/docs/connect-your-client/gui-clients/"
          target="_blank"
        >
          the documentation
        </Link>{' '}
        for more details.
      </Text>
    </Box>
  );
}

const AdvancedRoles = ({ gateway, isAdvancedOpen, setIsAdvancedOpen }: { gateway: Gateway, isAdvancedOpen: boolean, setIsAdvancedOpen: (isAdvancedOpen: boolean) => void } & PropsWithChildren) => {
  return (
    <Box mt={2} mb={2}>
      <ExpandToggle onClick={() => setIsAdvancedOpen(!isAdvancedOpen)}>
        {isAdvancedOpen ? (
          <ChevronDown size="small" />
        ) : (
          <ChevronRight size="small" />
        )}
        <Text fontSize={2} color="text.main">
          Advanced (Database roles)
        </Text>
      </ExpandToggle>
      {isAdvancedOpen && (
        <Box mt={2}>
          <Validation>
            <FieldSelect
              isMulti
              label="Database Roles"
              toolTipContent="These roles are determined by your Teleport role permissions and are read only."
              value={gateway.databaseRoles.map(role => ({
                value: role,
                label: role,
              }))}
              readOnly
              mb={0}
            />
          </Validation>
        </Box>
      )}
    </Box>
  );
};

const ExpandToggle = styled(Button).attrs({
  fill: 'minimal',
  size: 'small',
})`
  padding: 0;
  gap: ${props => props.theme.space[2]}px;

  &:hover {
    opacity: 0.8;
  }
`;
