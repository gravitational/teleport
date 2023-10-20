/*
Copyright 2019 Gravitational, Inc.

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

import React from 'react';
import PropTypes from 'prop-types';
import styled from 'styled-components';

import { Text, Alert, Card } from 'design';

export default function CardError(props) {
  return (
    <Card
      color="text.main"
      bg="levels.elevated"
      width="540px"
      mx="auto"
      my={6}
      p={5}
      {...props}
    />
  );
}

const Header = props => (
  <Text typography="h2" mb={4} textAlign="center" children={props.children} />
);

const Content = ({ message = '', desc = null }) => {
  const $errMessage = message ? (
    <Alert mt={2} mb={4}>
      {message}
    </Alert>
  ) : null;
  return (
    <>
      {$errMessage} {desc}
    </>
  );
};

export const NotFound = ({ message, ...rest }) => (
  <CardError {...rest}>
    <Header>404 Not Found</Header>
    <Content message={message} />
  </CardError>
);

export const AccessDenied = ({ message }) => (
  <CardError>
    <Header>Access Denied</Header>
    <Content message={message} />
  </CardError>
);

export const Failed = ({ message, ...rest }) => (
  <CardError {...rest}>
    <Header>Internal Error</Header>
    <Content message={message} />
  </CardError>
);

export const Offline = ({ message, title }) => (
  <CardError>
    <Header>{title}</Header>
    <Content
      desc={
        <Text typography="paragraph" textAlign="center">
          {message}
        </Text>
      }
    />
  </CardError>
);

Offline.propTypes = {
  title: PropTypes.string.isRequired,
  message: PropTypes.string,
};

export const LoginFailed = ({ message, loginUrl }) => (
  <CardError>
    <Header>Login Unsuccessful</Header>
    <Content
      message={message}
      desc={
        <Text typography="paragraph" textAlign="center">
          <HyperLink href={loginUrl}>Please attempt to log in again.</HyperLink>
        </Text>
      }
    />
  </CardError>
);

LoginFailed.propTypes = {
  message: PropTypes.string,
  loginUrl: PropTypes.string.isRequired,
};

const HyperLink = styled.a`
  color: ${({ theme }) => theme.colors.buttons.link.default};
`;
