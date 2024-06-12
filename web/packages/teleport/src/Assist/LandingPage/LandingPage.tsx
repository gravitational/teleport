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

import React from 'react';
import styled, { useTheme } from 'styled-components';

import {
  AuditLogIcon,
  PlusIcon,
  RemoteCommandIcon,
  SearchIcon,
  ServerIcon,
} from 'design/SVGIcon';

import Flex from 'design/Flex';

import Link from 'design/Link';

import { useAssist } from 'teleport/Assist/context/AssistContext';

const Container = styled.div`
  display: flex;
  justify-content: center;
`;

const Content = styled.div`
  background: ${p => p.theme.colors.levels.popout};
  color: ${p => p.theme.colors.text.main};
  font-size: 14px;
  padding: 20px 25px;
  border-radius: 7px;
  width: 700px;
`;

const Title = styled.h2`
  margin: 0 0 20px;
`;

const SubTitle = styled.h3`
  margin: 0 0 10px;
`;

const Features = styled.div`
  display: flex;
  flex-direction: column;
  margin-bottom: 20px;
`;

const Feature = styled.div`
  background: ${p => p.theme.colors.spotBackground[0]};
  margin-bottom: 10px;
  margin-right: 20px;
  padding: 10px 15px;
  border-radius: 5px;
  display: flex;
  align-items: center;
  font-size: 15px;

  svg {
    margin-right: 15px;

    path {
      fill: ${p => p.theme.colors.text.main};
    }
  }
`;

const Warning = styled.div`
  border: 2px solid ${p => p.theme.colors.warning.main};
  border-radius: 7px;
  padding: 10px 15px;
  margin-bottom: 15px;
`;

const NewChatButton = styled.div`
  padding: 10px 20px;
  border-radius: 7px;
  font-size: 15px;
  font-weight: bold;
  display: flex;
  cursor: pointer;
  margin: 0 15px;
  background: ${p => p.theme.colors.buttons.primary.default};
  color: ${p => p.theme.colors.buttons.primary.text};
  align-items: center;
  justify-content: center;

  svg {
    position: relative;
    margin-right: 10px;
  }

  &:hover {
    background: ${p => p.theme.colors.buttons.primary.hover};
  }
`;

export function LandingPage() {
  const theme = useTheme();

  const { createConversation } = useAssist();

  return (
    <Container>
      <Content>
        <Title>Teleport Assist</Title>

        <p>
          Teleport Assist utilizes facts about your infrastructure to help
          answer questions, generate command line scripts and help you perform
          routine tasks on target resources.
        </p>

        <Warning>
          Warning: This is an experimental{' '}
          <Link
            href="https://goteleport.com/legal/tos#product-previews"
            target="_blank"
            color={theme.colors.text.main}
          >
            Product Preview
          </Link>
          . The AI can hallucinate and produce harmful commands. Do not use in
          production. Let us know what you think in our{' '}
          <Link
            href="https://goteleport.com/slack"
            target="_blank"
            color={theme.colors.text.main}
          >
            community Slack.
          </Link>
        </Warning>

        <SubTitle>Features</SubTitle>

        <Features>
          <Feature>
            <ServerIcon size={24} /> Connect to your servers
          </Feature>
          <Feature>
            <RemoteCommandIcon size={24} /> Run commands across multiple nodes
          </Feature>
        </Features>

        <SubTitle>Coming Soon</SubTitle>

        <Features>
          <Feature>
            <AuditLogIcon size={24} />
            Analyze the audit log
          </Feature>
          <Feature>
            <SearchIcon size={24} />
            Interpret command outputs
          </Feature>
          <Feature>
            <PlusIcon size={24} />& much more!
          </Feature>
        </Features>

        <Flex justifyContent="center">
          <NewChatButton onClick={() => createConversation()}>
            <PlusIcon size={16} fill={theme.colors.buttons.primary.text} />
            Start a new conversation
          </NewChatButton>
        </Flex>
      </Content>
    </Container>
  );
}
