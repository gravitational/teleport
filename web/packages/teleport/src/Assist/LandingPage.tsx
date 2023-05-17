/*
Copyright 2023 Gravitational, Inc.

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

import React, { useCallback } from 'react';
import styled from 'styled-components';

import {
  AuditLogIcon,
  PlusIcon,
  RemoteCommandIcon,
  SearchIcon,
  ServerIcon,
} from 'design/SVGIcon';

import { useHistory } from 'react-router';

import Flex from 'design/Flex';

import Link from 'design/Link';

import cfg from 'teleport/config';
import { useConversations } from 'teleport/Assist/contexts/conversations';

const Container = styled.div`
  display: flex;
  flex: 1;
  justify-content: center;
`;

const Content = styled.div`
  background: #4a5688;
  color: white;
  font-size: 15px;
  padding: 20px 25px;
  border-radius: 7px;
  margin-top: 50px;
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
  background: rgba(255, 255, 255, 0.07);
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
      fill: white;
    }
  }
`;

const Warning = styled.div`
  border: 2px solid #ffab00;
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
  background: #9f85ff;
  color: black;
  align-items: center;
  justify-content: center;

  svg {
    position: relative;
    margin-right: 10px;
  }

  &:hover {
    background: #b29dff;
  }
`;

export function LandingPage() {
  const history = useHistory();

  const { create } = useConversations();

  const handleNewChat = useCallback(() => {
    create().then(conversationId =>
      history.push(cfg.getAssistConversationUrl(conversationId))
    );
  }, []);

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
            color="white"
          >
            Product Preview
          </Link>
          . The AI can hallucinate and produce harmful commands. Do not use in
          production. Let us know what you think in our{' '}
          <Link
            href="https://goteleport.com/slack"
            target="_blank"
            color="white"
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
          <NewChatButton onClick={() => handleNewChat()}>
            <PlusIcon size={16} fill="black" />
            Start a new conversation
          </NewChatButton>
        </Flex>
      </Content>
    </Container>
  );
}
