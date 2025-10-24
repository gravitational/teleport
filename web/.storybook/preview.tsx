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

import { Preview } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { initialize, mswLoader } from 'msw-storybook-addon';
import { ComponentType, PropsWithChildren } from 'react';
import { sb } from 'storybook/test';

import Box from '../packages/design/src/Box';
import { bblpTheme, darkTheme, lightTheme } from '../packages/design/src/theme';
import { Theme } from '../packages/design/src/theme/themes/types';
import { ConfiguredThemeProvider } from '../packages/design/src/ThemeProvider';
import cfg from '../packages/teleport/src/config';
import history from '../packages/teleport/src/services/history/history';
import { UserContextProvider } from '../packages/teleport/src/User';
import Logger, { ConsoleService } from '../packages/teleterm/src/logger';
import { StaticThemeProvider as TeletermThemeProvider } from '../packages/teleterm/src/ui/ThemeProvider';
import {
  darkTheme as teletermDarkTheme,
  lightTheme as teletermLightTheme,
} from '../packages/teleterm/src/ui/ThemeProvider/theme';

initialize(
  {
    onUnhandledRequest(request, print) {
      try {
        // Ignores asset related http requests, otherwise
        // it prints noisy warnings, hiding important ones.
        const url = new URL(request.url);
        if (
          url.pathname.startsWith('/sb-common-assets') ||
          url.pathname.startsWith('/index.json') ||
          url.pathname.startsWith('/.storybook') ||
          url.pathname.endsWith('.png') ||
          url.pathname.endsWith('.svg') ||
          url.pathname.endsWith('.css') ||
          url.pathname.endsWith('.yaml')
        ) {
          return;
        }
      } catch {
        /* empty */
      }

      print.warning();
    },
  },
  [
    // we emit these for posthog events (ignores any error),
    // and we don't ever mock them in stories.
    http.post(cfg.api.captureUserEventPath, () => {
      return HttpResponse.json({ message: 'ok' });
    }),
    http.post(cfg.api.capturePreUserEventPath, () => {
      return HttpResponse.json({ message: 'ok' });
    }),
  ]
);

sb.mock(import('../packages/teleport/src/services/recordings/metadata.ts'));

history.init();

Logger.init(new ConsoleService());

interface ThemeDecoratorProps {
  theme: string;
  title: string;
}

function ThemeDecorator(props: PropsWithChildren<ThemeDecoratorProps>) {
  let ThemeProvider: ComponentType<PropsWithChildren<{ theme: Theme }>>;
  let theme = darkTheme;

  if (props.title.startsWith('Teleterm/')) {
    ThemeProvider = TeletermThemeProvider;
    theme =
      props.theme === 'Dark Theme' ? teletermDarkTheme : teletermLightTheme;
  } else {
    ThemeProvider = ConfiguredThemeProvider;
    switch (props.theme) {
      case 'Dark Theme':
        theme = darkTheme;
        break;
      case 'Light Theme':
        theme = lightTheme;
        break;
      case 'BBLP Theme':
        theme = bblpTheme;
        break;
    }
  }

  return (
    <ThemeProvider theme={theme}>
      <Box p={3}>{props.children}</Box>
    </ThemeProvider>
  );
}

interface UserDecoratorProps {
  userContext?: boolean;
}

function UserDecorator(props: PropsWithChildren<UserDecoratorProps>) {
  if (props.userContext) {
    return <UserContextProvider>{props.children}</UserContextProvider>;
  }

  return props.children;
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
});

const preview: Preview = {
  args: {
    userContext: false,
  },
  parameters: {
    options: {
      storySort: {
        method: 'alphabetical',
        order: ['Teleport', 'TeleportE', 'Teleterm', 'Design', 'Shared'],
      },
    },
    controls: { expanded: true, disableSaveFromUI: true },
  },
  argTypes: { userContext: { table: { disable: true } } },
  loaders: [
    mswLoader,
    () => {
      queryClient.clear();
    },
  ],
  decorators: [
    (Story, meta) => (
      <QueryClientProvider client={queryClient}>
        <UserDecorator userContext={meta.args.userContext}>
          <ThemeDecorator theme={meta.globals.theme} title={meta.title}>
            <Story />
          </ThemeDecorator>
        </UserDecorator>
      </QueryClientProvider>
    ),
  ],
  globalTypes: {
    theme: {
      description: 'Global theme for components',
      defaultValue: 'Dark Theme',
      toolbar: {
        icon: 'contrast',
        items: ['Light Theme', 'Dark Theme', 'BBLP Theme'],
        dynamicTitle: true,
      },
    },
  },
};

export default preview;
