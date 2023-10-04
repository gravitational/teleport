/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Store } from 'shared/libs/stores';

type NoticeKinds = 'access-lists';

export type Notice = {
  kind: NoticeKinds;
  id: string;
  resourceName: string;
  date: Date;
  route: string;
};

export type NotificationState = {
  notices: Notice[];
};

const defaultNotificationState: NotificationState = {
  notices: [],
};

export class StoreNotifications extends Store<NotificationState> {
  state: NotificationState = defaultNotificationState;

  getNotifications() {
    return this.state.notices;
  }

  setNotifications(notices: Notice[]) {
    // Sort by earliest dates.
    const sortedNotices = notices.sort((a, b) => {
      return a.date.getTime() - b.date.getTime();
    });
    this.setState({ notices: [...sortedNotices] });
  }

  updateNotificationsByKind(notices: Notice[], kind: NoticeKinds) {
    switch (kind) {
      case 'access-lists':
        const filtered = this.state.notices.filter(
          n => n.kind !== 'access-lists'
        );
        this.setNotifications([...filtered, ...notices]);
    }
  }
}
