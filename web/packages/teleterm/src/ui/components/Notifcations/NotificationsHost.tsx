import React from 'react';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { Notifications } from './Notifications';

export function NotificationsHost() {
  const { notificationsService } = useAppContext();

  notificationsService.useState();

  return (
    <Notifications
      items={notificationsService.getNotifications()}
      onRemoveItem={item => notificationsService.removeNotification(item)}
    />
  );
}
