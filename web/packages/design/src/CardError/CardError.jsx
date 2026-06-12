/*
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

import PropTypes from 'prop-types';
import styled from 'styled-components';

import { Alert, Card, H1, P1 } from 'design';

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
  <H1 mb={4} textAlign="center" children={props.children} />
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
    <Content desc={<P1 textAlign="center">{message}</P1>} />
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
        <P1 textAlign="center">
          <HyperLink href={loginUrl}>Please attempt to log in again.</HyperLink>
        </P1>
      }
    />
  </CardError>
);

LoginFailed.propTypes = {
  message: PropTypes.string,
  loginUrl: PropTypes.string.isRequired,
};

export const LogoutFailed = ({ message, loginUrl }) => (
  <CardError>
    <Header>Logout Unsuccessful</Header>
    <Content
      message={message}
      desc={
        <P1 textAlign="center">
          <HyperLink href={loginUrl}>Return to login.</HyperLink>
        </P1>
      }
    />
  </CardError>
);

const HyperLink = styled.a`
  color: ${({ theme }) => theme.colors.buttons.link.default};
`;
