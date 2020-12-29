import React from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import { ButtonPrimary } from 'design';
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

export default function Container() {
  return <Workflow />;
}

export function Workflow() {
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>
          <Switch>
            <Route exact path={cfg.routes.requestNew}>
              <Breadcrumb trail="New Request" />
            </Route>
            <Route>
              <Breadcrumb />
            </Route>
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
        <Route exact path={cfg.routes.requestNew} component={RequestCreate} />
        <Route exact path={cfg.routes.requests} component={RequestList} />
      </Switch>
    </FeatureBox>
  );
}

function Breadcrumb({ trail }: { trail?: string }) {
  if (!trail) {
    return <>Access Requests</>;
  }

  return (
    <>
      <StyledLink to={cfg.routes.requests}>Access Requests</StyledLink>
      <Icons.ArrowRight mx={3} fontSize={2} />
      {trail}
    </>
  );
}

const StyledLink = styled(Link)`
  color: ${props => props.theme.colors.primary.contrastText};
  text-decoration: none;
`;
