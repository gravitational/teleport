/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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
import { Meta } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { llmApp } from './fixtures';
import { LLMAppConnectDialog as Component } from './LLMAppConnectDialog';

const meta: Meta = {
  title: 'Teleport/Apps/LLMAppConnectDialog',
  decorators: Story => {
    const ctx = createTeleportContext();
    return (
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <Story />
        </ContextProvider>
      </MemoryRouter>
    );
  },
};
export default meta;

export function LLMAppConnectDialog() {
  return <Component app={llmApp} onClose={() => {}} />;
}
