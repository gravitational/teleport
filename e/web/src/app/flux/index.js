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

import reactor from '../reactor';
import dialogsStore from './settingsDialogs/storeDialogs';
import authStore from './settingsAuth/store';
import rolesStore from './settingsRoles/store';
import clustersStore from './settingsClusters/store';
import licenseStatusStore from './license/store';

reactor.registerStores({
  'tlp_settings_dialogs': dialogsStore,
  'tlp_settings_auth': authStore,
  'tlp_settings_role': rolesStore,
  'tlp_settings_cluster': clustersStore,
  'tlp_license_status': licenseStatusStore
});
