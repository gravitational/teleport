import React from 'react';
import { Link } from 'react-router-dom';
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
        <FeatureHeaderTitle>
          <Switch>
            <Route exact path={cfg.getAccessRequestRoute()}>
              <Breadcrumb isList={true} />
            </Route>
            <Route exact path={cfg.routes.requestNew}>
              <Breadcrumb />
            </Route>
            <Route
              path={cfg.routes.requests}
              render={({ match }) => (
                <Breadcrumb trail={match.params.requestId} />
              )}
            />
          </Switch>
        </FeatureHeaderTitle>
        <ButtonPrimary
          as={Link}
          to={cfg.routes.requestNew}
          ml="auto"
          width="240px"
        >
          Request Access
        </ButtonPrimary>
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

function Breadcrumb({ isList, trail }: { isList?: boolean; trail?: string }) {
  if (isList) {
    return <>Access Requests</>;
  }

  return (
    <Flex alignItems="center">
      <StyledLink to={cfg.getAccessRequestRoute()}>Access Requests</StyledLink>
      <Icons.ArrowRight mx={3} fontSize={2} />
      {trail ? (
        <Flex mr={4} alignItems="baseline">
          <Text mr={3} title={'New Request'}>
            Request
          </Text>
          <Text typography="body1">{trail}</Text>
        </Flex>
      ) : (
        <Text mr={4} title={'New Request'}>
          New Request
        </Text>
      )}
    </Flex>
  );
}

const StyledLink = styled(Link)`
  color: ${props => props.theme.colors.primary.contrastText};
  text-decoration: none;
`;
