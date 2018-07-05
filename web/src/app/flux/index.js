/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import reactor from 'app/reactor';
import termStore from './terminal/store';
import userAcl from './userAcl/store';
import appStore from './app/appStore';
import nodeStore from './nodes/nodeStore';
import settingsStore from './settings/store';
import StatusStore from './status/statusStore';
import { register as registerSshHistory } from './sshHistory/store';
import { register as registerMisc } from './misc/store';
import { register as registerFileTransfer } from './fileTransfer';

registerSshHistory(reactor);
registerMisc(reactor);
registerFileTransfer(reactor);

reactor.registerStores({
  'tlpt_settings': settingsStore,
  'tlpt': appStore,
  'tlpt_terminal': termStore,
  'tlpt_nodes': nodeStore,
  'tlpt_user': require('./user/userStore'),
  'tlpt_user_invite': require('./user/userInviteStore'),
  'tlpt_user_acl': userAcl,
  'tlpt_sites': require('./sites/siteStore'),
  'tlpt_status': StatusStore,
  'tlpt_sessions_events': require('./sessions/eventStore'),
  'tlpt_sessions_archived': require('./sessions/archivedSessionStore'),
  'tlpt_sessions_active': require('./sessions/activeSessionStore'),
  'tlpt_sessions_filter': require('./storedSessionsFilter/storedSessionFilterStore')
});
