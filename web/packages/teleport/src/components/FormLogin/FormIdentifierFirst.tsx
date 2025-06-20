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

import {
  Box,
  ButtonLink,
  ButtonPrimary,
  Flex,
  Indicator,
  Input,
  Text,
} from 'design';
import * as Alerts from 'design/Alert';
import ButtonSso, { guessProviderType } from 'shared/components/ButtonSso';
import { useRefAutoFocus } from 'shared/hooks';
import { useAsync } from 'shared/hooks/useAsync';
import { AuthProvider } from 'shared/services';

import ResourceService from 'teleport/services/resources';
import { storageService } from 'teleport/services/storageService';

import SSOButtonList from './SsoButtons';

export type Props = {
  onLoginWithSso(provider: AuthProvider): void;
  /**
   * onUseLocalLogin is called to switch the view to the local login form.
   */
  onUseLocalLogin(): void;
};

export default function FormIdentifierFirst({
  onLoginWithSso,
  onUseLocalLogin,
}: Props) {
  const resourceService = new ResourceService();

  const [rememberedUsername, setRememberedUsername] = useState<string>(
    storageService.getRememberedSSOUsername()
  );
  const [username, setUsername] = useState<string>(rememberedUsername);
  const [connectors, setConnectors] = useState<AuthProvider[]>([]);

  useEffect(() => {
    if (rememberedUsername) {
      fetchMatchingConnectors(rememberedUsername);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const [fetchAttempt, fetchMatchingConnectors] = useAsync(
    useCallback(
      async (username: string) => {
        const connectors =
          await resourceService.getUserMatchedAuthConnectors(username);
        if (connectors.length === 0) {
          if (rememberedUsername) {
            // If we have a remembered username but no connectors, we clear the remembered username.
            storageService.clearRememberedSSOUsername();
            setRememberedUsername('');
            setUsername('');
            return;
          }
          throw new Error(`No SSO connectors found for user: ${username}`);
        }
        // If there isn't a remembered username, and there is only one matching connector, we take them straight to the IdP.
        if (connectors.length === 1 && !rememberedUsername) {
          onLoginWithSso(connectors[0]);
          storageService.setRememberedSSOUsername(username);
          setRememberedUsername(username);
          return;
        }
        setConnectors(connectors || []);
        setRememberedUsername(username);
        storageService.setRememberedSSOUsername(username);
        return;
      },
      [username]
    )
  );

  const onSubmitUsername = () => {
    fetchMatchingConnectors(username.trim());
  };

  const onNotYou = () => {
    storageService.clearRememberedSSOUsername();
    setUsername('');
    setRememberedUsername('');
    setConnectors([]);
  };

  const UsernamePrompt = () => {
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
      >
        <Text typography="h3" mb={3}>
          Enter your username to log in with SSO
        </Text>
        <Input
          ref={inputRef}
          value={username}
          onChange={e => setUsername(e.target.value)}
          placeholder="Username"
          width="80%"
          mb={3}
        />
        <ButtonPrimary
          type="submit"
          size="medium"
          disabled={fetchAttempt.status === 'processing' || !username.trim()}
          width="80%"
        >
          Log in
        </ButtonPrimary>
      </Flex>
    );
  };

  const MultipleConnectors = ({ providers }: { providers: AuthProvider[] }) => {
    return (
      <Flex flexDirection="column" gap={3}>
        <Text textAlign="center">Select an SSO provider to continue.</Text>
        {fetchAttempt.status === 'processing' && (
          <Box textAlign="center" m={4}>
            <Indicator delay="none" />
          </Box>
        )}
        {fetchAttempt.status === 'success' && (
          <SSOButtonList
            providers={providers}
            onClick={onLoginWithSso}
            isDisabled={false}
          />
        )}
        <Flex justifyContent="center">
          <ButtonLink style={{ padding: 0 }} onClick={onNotYou}>
            Not you? Click here.
          </ButtonLink>
        </Flex>
      </Flex>
    );
  };

  // OneConnector is the view for when there is a remembered user and only one connector for them.
  const OneConnector = ({ provider }: { provider: AuthProvider }) => {
    let { name, type, displayName } = provider;
    const connectorName = displayName || name;
    const ssoType = guessProviderType(connectorName, type);

    return (
      <Flex flexDirection="column" alignItems="center" gap={3} mt={3}>
        <ButtonSso
          title={`Log in with ${connectorName}`}
          ssoType={ssoType}
          disabled={fetchAttempt.status === 'processing'}
          autoFocus={true}
          onClick={e => {
            e.preventDefault();
            onLoginWithSso(provider);
          }}
        />
        <ButtonLink
          onClick={onNotYou}
          disabled={fetchAttempt.status === 'processing'}
        >
          Not you? Click here.
        </ButtonLink>
      </Flex>
    );
  };

  return (
    <Flex flexDirection="column" alignItems="center" width="100%">
      {fetchAttempt.status === 'error' && (
        <Alerts.Danger>{fetchAttempt.statusText}</Alerts.Danger>
      )}
      {!rememberedUsername ? (
        <>
          <UsernamePrompt />
          <ButtonLink
            mt={3}
            onClick={onUseLocalLogin}
            disabled={fetchAttempt.status === 'processing'}
          >
            Sign in another way
          </ButtonLink>
        </>
      ) : (
        <Text typography="h2" textAlign="center" mb={2}>
          Welcome back, {username}
        </Text>
      )}
      {connectors.length === 1 && <OneConnector provider={connectors[0]} />}
      {connectors.length > 1 && <MultipleConnectors providers={connectors} />}
    </Flex>
  );
}
