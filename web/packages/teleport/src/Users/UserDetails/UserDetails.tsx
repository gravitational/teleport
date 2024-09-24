import { Link, NavLink, useParams } from 'react-router-dom';

import { ButtonSecondary, Flex, H1, H3 } from 'design';
import { ArrowBack } from 'design/Icon';

import { useQuery } from '@tanstack/react-query';
import styled, { useTheme } from 'styled-components';
import React, { useEffect, useRef } from 'react';
import Box from 'design/Box';
import { AccessPath } from 'e-teleport/AccessGraph/AccessPath';

import api from 'teleport/services/api';
import { FeatureHeaderTitle } from 'teleport/components/Layout';
import cfg from 'teleport/config';

interface User {
  name: string;
  roles: string[];
  authType: string;
  allTraits: AllTraits;
  origin: string;
  isBot: boolean;
  traits: Traits;
}

interface AllTraits {
  db_users: string[];
  logins: string[];
}

interface Traits {
  logins: string[];
  databaseUsers: string[];
}

const TabsContainer = styled.div`
  position: relative;
  display: flex;
  gap: ${p => p.theme.space[5]}px;
  align-items: center;
  padding: 0 ${p => p.theme.space[5]}px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

const TabContainer = styled(NavLink)<{ selected?: boolean }>`
  padding: ${p => p.theme.space[1] + p.theme.space[2]}px
    ${p => p.theme.space[2]}px;
  position: relative;
  cursor: pointer;
  z-index: 2;
  opacity: ${p => (p.selected ? 1 : 0.5)};
  transition: opacity 0.3s linear;
  color: ${p => p.theme.colors.text.main};
  font-weight: 300;
  font-size: 16px;
  line-height: ${p => p.theme.space[5]}px;
  white-space: nowrap;
  text-decoration: none;

  &:hover {
    opacity: 1;
  }
`;

const TabBorder = styled.div`
  position: absolute;
  bottom: -1px;
  background: ${p => p.theme.colors.brand};
  height: 2px;
  transition: all 0.3s cubic-bezier(0.19, 1, 0.22, 1);
`;

enum Tab {
  Audit,
  AccessPath,
}

function getUser(username: string, signal?: AbortSignal): Promise<User> {
  return api.get(`/v1/webapi/users/${username}`, signal);
}

export function UserDetails() {
  const { username } = useParams<{ username: string; tab?: string }>();

  const theme = useTheme();

  const query = useQuery({
    queryKey: ['user', username],
    queryFn: ({ signal }) => getUser(username, signal),
  });

  const borderRef = useRef<HTMLDivElement>(null);
  const parentRef = useRef<HTMLDivElement>();

  const activeTab = Tab.AccessPath;

  useEffect(() => {
    if (!parentRef.current || !borderRef.current) {
      return;
    }

    const activeElement = parentRef.current.querySelector(
      `[data-tab-id="${activeTab}"]`
    );

    if (activeElement) {
      const parentBounds = parentRef.current.getBoundingClientRect();
      const activeBounds = activeElement.getBoundingClientRect();

      const left = activeBounds.left - parentBounds.left;
      const width = activeBounds.width;

      borderRef.current.style.left = `${left}px`;
      borderRef.current.style.width = `${width}px`;
    }
  }, [activeTab]);

  let content;
  if (activeTab === Tab.AccessPath) {
    content = (
      <Box
        id="access-path"
        width="100%"
        height="100%"
        borderTop="1px solid rgba(0, 0, 0, 0.5)"
      >
        <AccessPath
          name={username}
          kind="identity"
          background={theme.colors.levels.sunken}
        />
      </Box>
    );
  }

  return (
    <Flex flexDirection="column" height="100%">
      <Flex
        height="56px"
        paddingX="40px"
        alignItems="center"
        flex="0 0 56px"
        justifyContent="space-between"
      >
        <FeatureHeaderTitle as="div">
          <Flex alignItems="center">
            <ArrowBack
              data-testid="back-button"
              as={Link}
              mr={2}
              size="large"
              color="text.main"
              to={cfg.routes.users}
            />
            <H1>{username}</H1>

            <Box
              bg="rgba(255, 255, 255, 0.1)"
              lineHeight={1}
              borderRadius="12px"
              fontSize="13px"
              padding="4px 8px"
              marginLeft={3}
            >
              Local User
            </Box>
          </Flex>
        </FeatureHeaderTitle>

        <Flex gap={4}>
          <ButtonSecondary>Edit</ButtonSecondary>

          <ButtonSecondary>Reset Authentication</ButtonSecondary>
        </Flex>
      </Flex>

      <Flex marginY="16px">
        <Box paddingX="40px">
          <H3>Roles</H3>

          <Flex alignItems="center" flex="0 0 auto" gap={2} marginTop={2}>
            {query.data?.roles?.map(role => (
              <Box
                bg="rgba(255, 255, 255, 0.1)"
                lineHeight={1}
                borderRadius="12px"
                fontSize="13px"
                padding="4px 8px"
                key={role}
              >
                {role}
              </Box>
            ))}
          </Flex>
        </Box>
        {query.data?.traits?.logins?.length > 0 && (
          <Box paddingX="40px">
            <H3>Logins</H3>

            <Flex alignItems="center" flex="0 0 auto" gap={2} marginTop={2}>
              {query.data?.traits.logins.map(role => (
                <Box
                  bg="rgba(255, 255, 255, 0.1)"
                  lineHeight={1}
                  borderRadius="12px"
                  fontSize="13px"
                  padding="4px 8px"
                  key={role}
                >
                  {role}
                </Box>
              ))}
            </Flex>
          </Box>
        )}
        {query.data?.traits?.databaseUsers?.length > 0 && (
          <Box paddingX="40px">
            <H3>Database Users</H3>

            <Flex alignItems="center" flex="0 0 auto" gap={2} marginTop={2}>
              {query.data?.traits.databaseUsers.map(role => (
                <Box
                  bg="rgba(255, 255, 255, 0.1)"
                  lineHeight={1}
                  borderRadius="12px"
                  fontSize="13px"
                  padding="4px 8px"
                  key={role}
                >
                  {role}
                </Box>
              ))}
            </Flex>
          </Box>
        )}
      </Flex>

      <TabsContainer ref={parentRef}>
        <TabContainer
          data-tab-id={Tab.AccessPath}
          selected={activeTab === Tab.AccessPath}
          to={`/web/users/${username}/access-path`}
        >
          Access Path
        </TabContainer>

        <TabBorder ref={borderRef} />
      </TabsContainer>

      {content}
    </Flex>
  );
}
