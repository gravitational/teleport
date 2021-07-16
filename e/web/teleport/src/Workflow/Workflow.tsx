import React from 'react';
// eslint-disable-next-line import/named
import { Link, RouteComponentProps } from 'react-router-dom';
import styled from 'styled-components';
import { ButtonPrimary, Text, Flex } from 'design';
import * as Icons from 'design/Icon';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Route, Switch } from 'teleport/components/Router';
import cfg from 'e-teleport/config';
import RequestList from './RequestList';
import RequestCreate from './RequestCreate';
import RequestView from './RequestView';

export default function Workflow() {
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <Switch>
          <Route exact path={cfg.routes.requestNew} component={Header} />
          <Route path={cfg.routes.requests} component={Header} />
        </Switch>
      </FeatureHeader>
      <Switch>
        <Route
          exact
          path={cfg.getAccessRequestRoute()}
          component={RequestList}
        />
        <Route exact path={cfg.routes.requestNew} component={RequestCreate} />
        <Route path={cfg.routes.requests} component={RequestView} />
      </Switch>
    </FeatureBox>
  );
}

function Header({ match }: RouteComponentProps<{ requestId?: string }>) {
  if (match.url === cfg.getAccessRequestRoute()) {
    return (
      <>
        <FeatureHeaderTitle>Access Requests</FeatureHeaderTitle>
        <ButtonPrimary
          as={Link}
          to={cfg.routes.requestNew}
          ml="auto"
          width="240px"
        >
          Request Access
        </ButtonPrimary>
      </>
    );
  }

  const requestId = match.params?.requestId;
  return (
    <>
      <FeatureHeaderTitle>
        <Flex alignItems="center">
          <StyledLink to={cfg.getAccessRequestRoute()}>
            Access Requests
          </StyledLink>
          <Icons.ArrowRight mx={3} fontSize={2} />
          {requestId ? (
            <Flex mr={4} alignItems="baseline" overflow="hidden">
              <Text mr={3} title={'New Request'} style={{ flexShrink: 0 }}>
                Request
              </Text>
              <Text typography="body1" title={requestId}>
                {requestId}
              </Text>
            </Flex>
          ) : (
            <Text mr={4} title={'New Request'}>
              New Request
            </Text>
          )}
        </Flex>
      </FeatureHeaderTitle>
    </>
  );
}

const StyledLink = styled(Link)`
  color: ${props => props.theme.colors.primary.contrastText};
  text-decoration: none;
`;
