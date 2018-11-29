import { saveAuthProviderAttempt } from '../../flux/status/getters';

export default {
  saveAttempt: saveAuthProviderAttempt,
  store: ['tlp_settings_auth']
}
