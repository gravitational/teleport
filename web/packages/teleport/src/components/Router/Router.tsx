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

import React, { createContext, ReactNode, useContext, useEffect } from 'react';
import {
  createBrowserRouter,
  matchPath,
  Navigate,
  NavLink,
  Outlet,
  RouterProvider,
  Route as RouterRoute,
  Routes,
  UNSAFE_DataRouterContext,
  useBlocker,
  useLocation,
  useNavigate,
  useParams,
  type Location as RouterLocation,
} from 'react-router';

import { NotFound } from 'design/CardError';

import history from 'teleport/services/history';

// Re-export native React Router components for use throughout the app
export { NavLink, Outlet, useLocation, useParams };

/**
 * NoMatch component for 404 pages.
 */
const NoMatch = () => (
  <NotFound
    alignSelf="baseline"
    message="The requested path could not be found."
  />
);

/**
 * TitleSetter component sets the document title based on route params.
 */
function TitleSetter({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  const params = useParams();
  const clusterId = params.clusterId;

  useEffect(() => {
    if (title && clusterId) {
      document.title = `${clusterId} • ${title}`;
    } else if (title) {
      document.title = `${title}`;
    }
  }, [title, clusterId]);

  return <>{children}</>;
}

/**
 * Helper to wrap an element with TitleSetter if a title is provided.
 */
function withTitle(element: ReactNode, title?: string): ReactNode {
  if (title) {
    return <TitleSetter title={title}>{element}</TitleSetter>;
  }
  return element;
}

export interface RouteProps {
  path?: string;
  exact?: boolean;
  title?: string;
  component?: React.ComponentType;
  render?: () => ReactNode;
  element?: ReactNode;
  children?: ReactNode;
}

interface RouteBaseContextValue {
  basePattern: string;
}

const RouteBaseContext = createContext<RouteBaseContextValue>({
  basePattern: '',
});

function stripTrailingSplat(path: string) {
  if (path.endsWith('/*')) {
    return path.slice(0, -2);
  }
  if (path.endsWith('*')) {
    return path.slice(0, -1);
  }
  return path;
}

function joinPaths(base: string, child: string) {
  const normalizedBase = base.endsWith('/') ? base.slice(0, -1) : base;
  const normalizedChild = child.startsWith('/') ? child.slice(1) : child;
  if (!normalizedBase) {
    return `/${normalizedChild}`;
  }
  if (!normalizedChild) {
    return normalizedBase || '/';
  }
  return `${normalizedBase}/${normalizedChild}`;
}

function stripTrailingOptionalSegment(path: string) {
  return path.replace(/\/:[^/]+\?$/, '');
}

function RouteBaseProvider({
  path,
  children,
}: {
  path: string;
  children: ReactNode;
}) {
  const parent = useContext(RouteBaseContext);
  const normalized = stripTrailingSplat(path);
  let basePattern = parent.basePattern;

  if (normalized && normalized !== '*') {
    basePattern = normalized.startsWith('/')
      ? normalized
      : joinPaths(parent.basePattern || '', normalized);
  }

  return (
    <RouteBaseContext.Provider value={{ basePattern }}>
      {children}
    </RouteBaseContext.Provider>
  );
}

function RouteBasePassthrough({ children }: { children: ReactNode }) {
  const parent = useContext(RouteBaseContext);

  return (
    <RouteBaseContext.Provider value={parent}>
      {children}
    </RouteBaseContext.Provider>
  );
}

/**
 * Route component - a thin wrapper that will be processed by Switch.
 * This maintains compatibility with legacy route definitions.
 */
const Route: React.FC<RouteProps> = () => {
  // This component is processed by Switch, not rendered directly.
  return null;
};

interface SwitchProps {
  children: ReactNode;
}

/**
 * Switch component that converts legacy Route children to Routes.
 * Handles prefix stripping for nested routes.
 */
const Switch = ({ children }: SwitchProps) => {
  const elements: ReactNode[] = [];
  let hasWildcard = false;

  const { basePattern } = useContext(RouteBaseContext);
  const childElements = React.Children.toArray(children).filter(child =>
    React.isValidElement(child)
  ) as React.ReactElement[];
  const effectiveBasePattern =
    basePattern && basePattern !== '/' ? basePattern : '';

  childElements.forEach((child, index) => {
    const props = child.props as RouteProps;
    const {
      component: Component,
      render,
      element,
      path,
      exact,
      title,
      children: childContent,
    } = props;

    // Determine the element to render
    let routeElement: ReactNode;
    if (element) {
      routeElement = element;
    } else if (Component) {
      routeElement = <Component />;
    } else if (render) {
      routeElement = render();
    } else if (childContent) {
      routeElement = childContent;
    }

    // Apply title wrapper
    routeElement = withTitle(routeElement, title);

    // Determine path - in v7, we need to handle prefix stripping for nested routes
    let routePath = path;

    if (!path) {
      // No path means catch-all
      routePath = '*';
      hasWildcard = true;
    } else {
      // Add /* suffix for non-exact routes to match sub-paths
      if (exact !== true && !path.endsWith('/*') && !path.endsWith('*')) {
        routePath = path.endsWith('/') ? `${path}*` : `${path}/*`;
      }

      // Strip parent pathname base from absolute paths to make them relative
      // This is needed for nested Routes to work correctly in v7
      const canStripBase =
        effectiveBasePattern &&
        effectiveBasePattern !== '/' &&
        routePath.startsWith('/');

      if (canStripBase) {
        const match = matchPath(
          { path: effectiveBasePattern, end: false },
          routePath
        );
        const trimmedPattern =
          stripTrailingOptionalSegment(effectiveBasePattern);
        const trimmedMatch =
          trimmedPattern !== effectiveBasePattern
            ? matchPath({ path: trimmedPattern, end: false }, routePath)
            : null;
        const matchStripped = match
          ? routePath.slice(match.pathnameBase.length)
          : '';
        const trimmedStripped = trimmedMatch
          ? routePath.slice(trimmedMatch.pathnameBase.length)
          : '';
        let effectiveMatch = match;

        if (match && trimmedMatch) {
          const matchEmpty = matchStripped === '' || matchStripped === '/';
          const trimmedEmpty =
            trimmedStripped === '' || trimmedStripped === '/';

          if (matchEmpty && !trimmedEmpty) {
            effectiveMatch = trimmedMatch;
          } else if (!matchEmpty && trimmedEmpty) {
            effectiveMatch = match;
          } else if (!matchEmpty && !trimmedEmpty) {
            effectiveMatch =
              trimmedStripped.length < matchStripped.length
                ? trimmedMatch
                : match;
          }
        } else if (trimmedMatch && !match) {
          effectiveMatch = trimmedMatch;
        }

        if (effectiveMatch) {
          const effectiveStripped = routePath.slice(
            effectiveMatch.pathnameBase.length
          );
          if (effectiveStripped === '' || effectiveStripped === '/') {
            // Route matches parent exactly - use * to catch all
            routePath = '*';
          } else if (effectiveStripped.startsWith('/')) {
            routePath = effectiveStripped.slice(1); // Remove leading slash
          } else {
            routePath = effectiveStripped;
          }
        }
      }
    }

    if (routePath === '*') {
      hasWildcard = true;
    }

    if (routeElement) {
      routeElement =
        routePath && routePath !== '*' ? (
          <RouteBaseProvider path={routePath}>{routeElement}</RouteBaseProvider>
        ) : (
          <RouteBasePassthrough>{routeElement}</RouteBasePassthrough>
        );
    }

    elements.push(
      <RouterRoute
        key={path || `route-${index}`}
        path={routePath}
        element={routeElement}
      />
    );
  });

  // Add catch-all 404 if no wildcard route exists
  if (!hasWildcard) {
    elements.push(
      <RouterRoute key="not-found" path="*" element={<NoMatch />} />
    );
  }

  return <Routes>{elements}</Routes>;
};

interface RedirectProps {
  to: string;
  from?: string;
  path?: string;
}

/**
 * Redirect component - uses Navigate in v7.
 */
const Redirect = ({ to }: RedirectProps) => {
  return <Navigate to={to} replace />;
};

interface PromptProps {
  when: boolean;
  message: string | ((nextLocation: RouterLocation) => string | boolean);
}

function DataRouterPrompt({ when, message }: PromptProps) {
  const blocker = useBlocker(when);

  useEffect(() => {
    if (blocker.state !== 'blocked') {
      return;
    }

    const confirmation =
      typeof message === 'function' ? message(blocker.location) : message;

    if (confirmation === true) {
      blocker.proceed();
      return;
    }

    if (confirmation === false) {
      blocker.reset();
      return;
    }

    if (window.confirm(confirmation)) {
      blocker.proceed();
    } else {
      blocker.reset();
    }
  }, [blocker, message]);

  return null;
}

/**
 * Prompt component for blocking navigation.
 */
function Prompt({ when, message }: PromptProps) {
  const dataRouterContext = React.useContext(UNSAFE_DataRouterContext);

  useEffect(() => {
    if (!when) return;

    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      // Modern browsers ignore custom messages but still show a prompt
      const text =
        typeof message === 'string'
          ? message
          : 'You have unsaved changes. Are you sure you want to leave?';
      e.returnValue = text;
      return text;
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [when, message]);

  return dataRouterContext ? (
    <DataRouterPrompt when={when} message={message} />
  ) : null;
}

/**
 * Component that captures the navigate function and initializes the history service.
 * This must be rendered inside BrowserRouter.
 */
function NavigationInitializer({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    // Initialize history service with navigation functions
    history.init({
      navigate,
      getLocation: () => location,
    });
  }, [navigate, location]);

  return <>{children}</>;
}

/**
 * Router component that wraps the app with a data router and initializes navigation.
 */
function Router({ children }: { children: ReactNode }) {
  const router = React.useMemo(
    () =>
      createBrowserRouter([
        {
          path: '*',
          element: (
            <RouteBaseContext.Provider value={{ basePattern: '' }}>
              <NavigationInitializer>{children}</NavigationInitializer>
            </RouteBaseContext.Provider>
          ),
        },
      ]),
    [children]
  );

  return <RouterProvider router={router} />;
}

export { Route, Router, Switch, Redirect, NoMatch, Prompt };
