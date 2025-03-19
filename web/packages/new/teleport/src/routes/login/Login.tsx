import { useLocalStorage } from 'usehooks-ts';

import { KeysEnum } from '../../services/storage/types';

export function NewLogin() {
  const [licenseAcknowledged] = useLocalStorage(
    KeysEnum.LICENSE_ACKNOWLEDGED,
    false
  );

  if (!licenseAcknowledged) {
    return null;
  }

  return <div>hello there!</div>;
}
