import { Recovery } from './Recovery';
import { State } from './useRecovery';

export default {
  title: 'TeleportE/Account/Recovery',
};

export const Loaded = () => <Recovery {...props} />;

export const LoadedFirstTime = () => (
  <Recovery {...props} userHasCodes={false} />
);

export const LoadedNoEmailAddress = () => (
  <Recovery {...props} userHasCodes={false} isRecoveryEnabled={false} />
);

export const Processing = () => (
  <Recovery {...props} attempt={{ status: 'processing' }} />
);

const props: State = {
  attempt: { status: 'success' },
  token: '',
  createdDateText: '10/9/2020',
  userHasCodes: true,
  setToken: () => null,
  isReAuthenticateVisible: false,
  isCodesVisible: false,
  showReAuthenticate: () => null,
  hideReAuthenticate: () => null,
  hideCodes: () => null,
  fetchCreatedDate: () => null,
  isRecoveryEnabled: true,
};
