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

import { act, renderHook, waitFor } from '@testing-library/react';

import { ShieldCheck } from 'design/Icon';

import {
  ToastNotificationProvider,
  useToastNotifications,
} from './ToastNotificationContext';
import { ToastNotificationItemObjectContent } from './types';

const wrapper = ({ children }) => (
  <ToastNotificationProvider>{children}</ToastNotificationProvider>
);

describe('useToastNotification', () => {
  test('add notification', async () => {
    const { result } = renderHook(() => useToastNotifications(), {
      wrapper,
    });

    expect(result.current.notifications).toHaveLength(0);

    // addNotification with content type "string"
    await act(async () => {
      result.current.addNotification('error', 'error content');
    });

    await waitFor(() => {
      expect(result.current.notifications).toHaveLength(1);
    });

    expect(result.current.notifications[0].content).toEqual('error content');
    expect(result.current.notifications[0].severity).toEqual('error');
    expect(result.current.notifications[0].id).toBeTruthy();

    // addNotification with content type "Object"
    await act(async () => {
      result.current.addNotification('info', {
        title: 'some title',
        description: 'some description',
        icon: ShieldCheck,
      });
    });

    await waitFor(() => {
      expect(result.current.notifications).toHaveLength(2);
    });

    expect(result.current.notifications[1].id).toBeTruthy();
    expect(result.current.notifications[1].severity).toEqual('info');

    const item = result.current.notifications[1]
      .content as ToastNotificationItemObjectContent;
    expect(item.title).toEqual('some title');
    expect(item.description).toEqual('some description');
    expect(item.icon).toEqual(ShieldCheck);
  });

  test('remove notification', async () => {
    const { result } = renderHook(() => useToastNotifications(), {
      wrapper,
    });

    await act(async () => {
      result.current.addNotification('error', 'error content');
    });

    await waitFor(() => {
      expect(result.current.notifications).toHaveLength(1);
    });

    await act(async () => {
      result.current.removeNotification(result.current.notifications[0].id);
    });

    await waitFor(() => {
      expect(result.current.notifications).toHaveLength(0);
    });
  });
});
