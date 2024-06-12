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

import React, { useEffect, useState } from 'react';
import styled from 'styled-components';
import Flex from 'design/Flex';

import Select from 'shared/components/Select';
import { ButtonPrimary, ButtonSecondary } from 'design';

import { useAssist } from 'teleport/Assist/context/AssistContext';
import { getLoginsForQuery } from 'teleport/Assist/service';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { useUser } from 'teleport/User/UserContext';

interface ExecuteRemoteCommandEntryProps {
  command: string;
  query: string;
  disabled: boolean;
}

const Container = styled.div`
  padding: 15px 15px 15px 17px;
  width: var(--command-input-width);
`;

const StyledInput = styled.input<{ hasError: boolean }>`
  border: 1px solid
    ${p =>
      p.hasError
        ? p.theme.colors.error.main
        : p.theme.colors.spotBackground[0]};
  padding: 12px 15px;
  border-radius: 5px;
  font-family: ${p => p.theme.fonts.mono};
  background: ${p => p.theme.colors.levels.surface};

  &:disabled {
    background: ${p => p.theme.colors.spotBackground[0]};
  }

  &:active:not(:disabled),
  &:focus:not(:disabled) {
    outline: none;
    border-color: ${p => p.theme.colors.text.slightlyMuted};
  }
`;

const QueryInput = styled(StyledInput)`
  flex: 1;
`;

const ErrorMessage = styled.div`
  color: ${p => p.theme.colors.error.main};
`;

const CommandInput = styled(StyledInput)`
  width: calc(100% - 32px);
`;

const Spacer = styled.div`
  padding: 0 10px;
`;

const InfoText = styled.span`
  display: block;
  font-size: 14px;
  font-weight: 600;
  margin: 5px 0;
`;

export function ExecuteRemoteCommandEntry(
  props: ExecuteRemoteCommandEntryProps
) {
  const { preferences } = useUser();
  const { executeCommand } = useAssist();

  const [hasRan, setHasRan] = useState(false);
  const [command, setCommand] = useState(props.command);
  const [query, setQuery] = useState(props.query);
  const [selectedLogin, setSelectedLogin] = useState('');
  const [availableLogins, setAvailableLogins] = useState<string[]>([]);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const { clusterId } = useStickyClusterId();

  async function updateAvailableLogins() {
    if (props.disabled) {
      return;
    }

    setErrorMessage(null);

    try {
      const logins = await getLoginsForQuery(query, clusterId);

      const preferredLogin = logins.find(login =>
        preferences.assist.preferredLogins.includes(login)
      );
      if (preferredLogin) {
        setSelectedLogin(preferredLogin);
      } else if (!selectedLogin || !logins.includes(selectedLogin)) {
        setSelectedLogin(logins[0]);
      }

      setAvailableLogins(logins);
    } catch (err) {
      if (err instanceof Error) {
        if (err.message.includes('failed to parse predicate expression')) {
          setErrorMessage('Invalid query');

          return;
        }

        setErrorMessage(err.message);

        return;
      }

      setErrorMessage('Something went wrong');
    }
  }

  useEffect(() => {
    updateAvailableLogins();
  }, []);

  function handleQueryChange(event: React.ChangeEvent<HTMLInputElement>) {
    setQuery(event.target.value);
  }

  function handleCommandChange(event: React.ChangeEvent<HTMLInputElement>) {
    setCommand(event.target.value);
  }

  function handleReset() {
    setQuery(props.query);
    setCommand(props.command);
    updateAvailableLogins();
  }

  const disabled =
    errorMessage !== null || !selectedLogin || !command || props.disabled;

  function handleRun() {
    if (disabled) {
      return;
    }

    setHasRan(true);
    executeCommand(selectedLogin, command, query);
  }

  return (
    <Container>
      <InfoText style={{ marginTop: 0 }}>
        Teleport would like to connect to
      </InfoText>

      <Flex justifyContent="space-between" alignItems="center">
        <QueryInput
          hasError={errorMessage !== null}
          value={query}
          onChange={handleQueryChange}
          onBlur={updateAvailableLogins}
          disabled={hasRan || props.disabled}
        />

        <Spacer>as</Spacer>

        <Select
          onChange={event => setSelectedLogin(event['value'])}
          isDisabled={hasRan || props.disabled}
          value={{ value: selectedLogin, label: selectedLogin }}
          options={availableLogins.map(option => {
            return { label: option, value: option };
          })}
          css={'width: 150px;'}
        />
      </Flex>

      {errorMessage && <ErrorMessage>{errorMessage}</ErrorMessage>}

      <InfoText>and run</InfoText>

      <CommandInput
        hasError={!command}
        value={command}
        onChange={handleCommandChange}
        disabled={hasRan || props.disabled}
      />

      {!command && <ErrorMessage>Command is required</ErrorMessage>}

      {!hasRan && !props.disabled && (
        <Flex mt={3} justifyContent="flex-end">
          <ButtonSecondary onClick={handleReset}>Reset</ButtonSecondary>

          <ButtonPrimary ml={3} onClick={handleRun} disabled={disabled}>
            Run
          </ButtonPrimary>
        </Flex>
      )}
    </Container>
  );
}
