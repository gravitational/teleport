import { MemoryRouter } from 'react-router';

import Flex from 'design/Flex';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';
import { notifications } from 'teleport/Notifications/fixtures';
import { Notification } from 'teleport/Notifications/Notification';

export default {
  title: 'TeleportE/Notifications',
};

export const NotificationsTypesE = () => {
  const ctx = createTeleportContextE();

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Flex
          p={4}
          gap={4}
          css={`
            background: ${props => props.theme.colors.levels.surface};
            width: fit-content;
            height: fit-content;
            flex-direction: column;
          `}
        >
          {notifications.map(notification => {
            return (
              <Notification
                notification={notification}
                key={notification.id}
                closeNotificationsList={() => null}
                markNotificationAsClicked={() => null}
                removeNotification={() => null}
              />
            );
          })}
        </Flex>
      </ContextProvider>
    </MemoryRouter>
  );
};
