/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import type { Meta } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router';

import { ToastNotificationProvider } from 'shared/components/ToastNotification';

import { Aws } from './Aws';

export const Flow = () => {
  return (
    <MemoryRouter>
      <ToastNotificationProvider>
        <Aws />
      </ToastNotificationProvider>
    </MemoryRouter>
  );
};

const meta: Meta<typeof Flow> = {
  title: 'Teleport/Integrations/Enroll/Cloud/Aws',
  component: Flow,
};

export default meta;
