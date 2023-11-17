import React, { lazy, Suspense, useEffect, useRef } from 'react';
import styled from 'styled-components';

import { Box } from 'design';
import { Redirect, Route, Switch } from 'teleport/components/Router';

import { useLocation } from 'react-router';
import { NavLink } from 'react-router-dom';

import { useTeleport } from 'teleport';

import config from 'e-teleport/config';

// keep the report list in the bundle as it's the default page
import { ReportList } from 'e-teleport/AccessMonitoring/ReportList';

const Report = lazy(() => import('e-teleport/AccessMonitoring/Report'));
const QueryEditor = lazy(
  () => import('e-teleport/AccessMonitoring/QueryEditor')
);

const Container = styled.div``;

const TabsContainer = styled.div`
  position: relative;
  display: flex;
  gap: ${p => p.theme.space[5]}px;
  align-items: center;
  padding: 0 ${p => p.theme.space[5]}px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

const TabContainer = styled(NavLink)`
  padding: ${p => p.theme.space[1] + p.theme.space[2]}px
    ${p => p.theme.space[2]}px;
  position: relative;
  cursor: pointer;
  z-index: 2;
  opacity: ${p => (p.selected ? 1 : 0.5)};
  transition: opacity 0.3s linear;
  color: ${p => p.theme.colors.text.main};
  font-weight: 300;
  font-size: 22px;
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
  Reports,
  QueryEditor,
}

export function AccessMonitoring() {
  const ctx = useTeleport();

  const location = useLocation();

  const borderRef = useRef<HTMLDivElement>(null);
  const parentRef = useRef<HTMLElement>();

  const activeTab =
    location.pathname === config.routes.accessMonitoring.queryEditor
      ? Tab.QueryEditor
      : Tab.Reports;

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

  const isReportsPage =
    location.pathname !== config.routes.accessMonitoring.queryEditor;

  const hasQueryAccess = ctx.storeUser.getAuditQueryAccess().list;
  const hasReportAccess = ctx.storeUser.getSecurityReportAccess().list;

  if (!hasReportAccess && isReportsPage) {
    return <Redirect to={config.routes.accessMonitoring.queryEditor} />;
  }

  return (
    <Container>
      <TabsContainer ref={parentRef}>
        {hasReportAccess && (
          <TabContainer
            data-tab-id={Tab.Reports}
            selected={activeTab === Tab.Reports}
            to={config.routes.accessMonitoring.base}
          >
            Reports
          </TabContainer>
        )}
        {hasQueryAccess && (
          <TabContainer
            data-tab-id={Tab.QueryEditor}
            selected={activeTab === Tab.QueryEditor}
            to={config.routes.accessMonitoring.queryEditor}
          >
            Query Editor
          </TabContainer>
        )}

        <TabBorder ref={borderRef} />
      </TabsContainer>

      <Box p={6}>
        <Suspense fallback={null}>
          <Switch>
            {hasQueryAccess && (
              <Route
                path={config.routes.accessMonitoring.queryEditor}
                component={QueryEditor}
              />
            )}

            {hasReportAccess && (
              <Switch>
                <Route
                  path={config.routes.accessMonitoring.report}
                  component={Report}
                />

                <Route
                  path={config.routes.accessMonitoring.base}
                  component={ReportList}
                />
              </Switch>
            )}
          </Switch>
        </Suspense>
      </Box>
    </Container>
  );
}
