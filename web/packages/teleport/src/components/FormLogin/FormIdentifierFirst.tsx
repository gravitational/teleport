/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { useCallback, useEffect, useState } from 'react';
import styled from 'styled-components';

import {
  Box,
  ButtonPrimary,
  ButtonText,
  Card,
  Flex,
  Indicator,
  Input,
  LabelInput,
  Text,
} from 'design';
import * as Alerts from 'design/Alert';
import ButtonSso, { guessProviderType } from 'shared/components/ButtonSso';
import { useRefAutoFocus } from 'shared/hooks';
import { Attempt, useAsync } from 'shared/hooks/useAsync';
import { AuthProvider } from 'shared/services';

import ResourceService from 'teleport/services/resources';
import { storageService } from 'teleport/services/storageService';

type Props = {
  onLoginWithSso(provider: AuthProvider): void;
  /**
   * onUseLocalLogin is called to switch the view to the local login form.
   */
  onUseLocalLogin(): void;
  /**
   * isLocalAuthEnabled is whether local auth is enabled. If not, the identifier first login screen should be the only possible screen.
   */
  isLocalAuthEnabled: boolean;
  /**
   * title is the title of the form.
   */
  title: string;
  ssoTitle: string;
};

/**
 * FormIdentifierFirst is the login form for identifier-first login.
 */
export function FormIdentifierFirst({
  onLoginWithSso,
  onUseLocalLogin,
  isLocalAuthEnabled,
  title,
  ssoTitle,
}: Props) {
  const [resourceService] = useState(() => new ResourceService());

  const [rememberedUsername, setRememberedUsername] = useState<string>(
    storageService.getRememberedSsoUsername().trim()
  );
  const [username, setUsername] = useState<string>(rememberedUsername);
  const [connectors, setConnectors] = useState<AuthProvider[]>([]);

  useEffect(() => {
    if (rememberedUsername) {
      fetchMatchingConnectors(rememberedUsername);
    }
  }, [rememberedUsername]);

  const [fetchAttempt, fetchMatchingConnectors] = useAsync(
    useCallback(
      async (username: string) => {
        const matchedConnectors =
          await resourceService.getUserMatchedAuthConnectors(username);
        if (matchedConnectors.length === 0) {
          if (rememberedUsername) {
            // If we have a remembered username but no connectors, we clear the remembered username.
            storageService.clearRememberedSsoUsername();
            setRememberedUsername('');
            setUsername('');
            return;
          }
          throw new Error(`No SSO connectors found for user: ${username}`);
        }
        // If there isn't a remembered username, and there is only one matching connector, we take them straight to the IdP.
        if (matchedConnectors.length === 1 && !rememberedUsername) {
          onLoginWithSso(matchedConnectors[0]);
          storageService.setRememberedSsoUsername(username);
          setRememberedUsername(username);
          return;
        }
        setConnectors(matchedConnectors);
        setRememberedUsername(username);
        storageService.setRememberedSsoUsername(username);
        return;
      },
      [username]
    )
  );

  const onSubmitUsername = () => {
    fetchMatchingConnectors(username);
  };

  const onNotYou = () => {
    storageService.clearRememberedSsoUsername();
    setUsername('');
    setRememberedUsername('');
    setConnectors([]);
  };

  return (
    <Card my="5" mx="auto" width={650} p={4}>
      <Flex flexDirection="column" alignItems="center">
        <Text typography="h1" textAlign="center">
          {rememberedUsername ? title : ssoTitle}
        </Text>
        {fetchAttempt.status === 'error' && (
          <Alerts.Danger mt={3}>{fetchAttempt.statusText}</Alerts.Danger>
        )}
        <Flex flexDirection="column" alignItems="center" width="100%">
          {rememberedUsername ? (
            <Text typography="h2" textAlign="center" mt={1}>
              Welcome, {username}
            </Text>
          ) : (
            <>
              <UsernamePrompt
                onSubmitUsername={onSubmitUsername}
                username={username}
                setUsername={setUsername}
                fetchAttempt={fetchAttempt}
              />
              {isLocalAuthEnabled && (
                <ViewSwitchButton
                  onClick={onUseLocalLogin}
                  disabled={fetchAttempt.status === 'processing'}
                >
                  Other Sign-in Options
                </ViewSwitchButton>
              )}
            </>
          )}
          {fetchAttempt.status === 'processing' && !!rememberedUsername && (
            <Box textAlign="center" m={4}>
              <Indicator delay="none" />
            </Box>
          )}
          {fetchAttempt.status === 'success' && connectors.length > 0 && (
            <ConnectorList
              providers={connectors}
              onLoginWithSso={onLoginWithSso}
              onNotYou={onNotYou}
            />
          )}
        </Flex>
      </Flex>
    </Card>
  );
}

function UsernamePrompt({
  onSubmitUsername,
  username,
  setUsername,
  fetchAttempt,
}: {
  onSubmitUsername(): void;
  username: string;
  setUsername: (username: string) => void;
  fetchAttempt: Attempt<void>;
}) {
  const inputRef = useRefAutoFocus<HTMLInputElement>({
    shouldFocus: true,
  });

  return (
    <Flex
      as="form"
      alignItems="center"
      justifyContent="center"
      flexDirection="column"
      onSubmit={e => {
        e.preventDefault();
        onSubmitUsername();
      }}
      width="100%"
      gap={3}
      mb={3}
      mt={5}
    >
      <Flex flexDirection="column" alignItems="flex-start" width="100%">
        <LabelInput>Username</LabelInput>
        <Input
          ref={inputRef}
          value={username}
          onChange={e => setUsername(e.target.value?.trim())}
          placeholder="Username or email address"
          width="100%"
          autoFocus={true}
        />
      </Flex>
      <ButtonPrimary
        type="submit"
        size="extra-large"
        disabled={fetchAttempt.status === 'processing' || !username}
        width="100%"
      >
        Next
      </ButtonPrimary>
    </Flex>
  );
}

function ConnectorList({
  providers,
  onLoginWithSso,
  onNotYou,
}: {
  providers: AuthProvider[];
  onLoginWithSso(provider: AuthProvider): void;
  onNotYou(): void;
}) {
  const $btns = providers.map((item, index) => {
    let { name, type, displayName } = item;
    const title = displayName || name;
    const ssoType = guessProviderType(title, type);
    return (
      <StyledButtonSso
        px={5}
        size="extra-large"
        key={index}
        title={`Sign in with ${title}`}
        ssoType={ssoType}
        onClick={e => {
          e.preventDefault();
          onLoginWithSso(item);
        }}
      />
    );
  });

  return (
    <Flex flexDirection="column" gap={2} mt={5} width="100%">
      <Text textAlign="start" typography="subtitle1" color="text.muted">
        Select an identity provider to continue:
      </Text>
      {$btns}
      <Flex justifyContent="center" mt={3}>
        <ViewSwitchButton onClick={onNotYou} disabled={false}>
          Sign in with a different account
        </ViewSwitchButton>
      </Flex>
    </Flex>
  );
}

/**
 * ViewSwitch button is the button used to switch between login form views.
 */
export function ViewSwitchButton({
  onClick,
  disabled,
  children,
}: {
  onClick(): void;
  disabled: boolean;
} & React.PropsWithChildren) {
  return (
    <ButtonText
      size="large"
      onClick={onClick}
      disabled={disabled}
      css={`
        color: ${props => props.theme.colors.text.muted};
        background: transparent;
        &:hover {
          background: transparent;
        }
      `}
    >
      {children}
    </ButtonText>
  );
}

const StyledButtonSso = styled(ButtonSso)`
  width: 100%;
  justify-content: flex-start;
  padding-top: 12px;
  padding-bottom: 12px;
`;
