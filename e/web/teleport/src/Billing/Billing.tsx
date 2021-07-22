import React from 'react';
import styled from 'styled-components';
import { Box } from 'design';
import { Route, Switch, NavLink } from 'teleport/components/Router';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'e-teleport/config';
import Usage from './Usage';

export default function Billing() {
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>
          <TabItem as={NavLink} to={cfg.routes.billingUsage}>
            Usage
          </TabItem>
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Box mt={3}>
        <Switch>
          <Route path={cfg.routes.billingUsage} component={Usage} />
        </Switch>
      </Box>
    </FeatureBox>
  );
}

const TabItem = styled.button`
  color: ${props => props.theme.colors.text.secondary};
  cursor: pointer;
  display: inline-flex;
  font-size: 14px;
  padding: 12px 40px;
  position: relative;
  text-decoration: none;
  font-weight: 500;

  &:hover {
    background: ${props =>
      props.active
        ? props.theme.colors.primary.light
        : 'rgba(255, 255, 255, .06)'};
  }

  &.active {
    color: ${props => props.theme.colors.light};
  }

  &.active:after {
    background-color: ${props => props.theme.colors.accent};
    content: '';
    position: absolute;
    bottom: 0;
    left: 0;
    width: 100%;
    height: 4px;
  }
`;
