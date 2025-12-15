/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import styled from 'styled-components';

import { Failed } from 'design/CardError';

import { StaticThemeProvider } from 'teleterm/ui/ThemeProvider';
import { darkTheme } from 'teleterm/ui/ThemeProvider/theme';

export const StyledApp = styled.div`
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  position: absolute;
  display: flex;
  flex-direction: column;
`;

export const FailedApp = (props: { message: string }) => {
  return (
    <StyledApp>
      {/*
        FailedApp is used above ThemeProvider in the component hierarchy. Since it cannot depend on
        ThemeProvider to provide a theme, it needs to use StaticThemeProvider to provide one.
      */}
      <StaticThemeProvider theme={darkTheme}>
        <Failed
          message={props.message}
          alignSelf={'baseline'}
          width="600px"
          css={`
            white-space: pre-wrap;
          `}
        />
      </StaticThemeProvider>
    </StyledApp>
  );
};
