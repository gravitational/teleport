import useAttempt from 'shared/hooks/useAttemptNext';

import RecoveryService, {
  StartRecoveryRequest,
} from 'e-teleport/services/recovery';

export default function useRecoveryStart({
  recoveryType,
  recoveryService,
}: Props) {
  const { attempt, run } = useAttempt('');

  function submit(data: StartRecoveryRequest) {
    run(() => recoveryService.startRecovery(data));
  }

  return {
    submit,
    attempt,
    recoveryType,
  };
}

type Props = {
  recoveryService: RecoveryService;
  recoveryType: RecoveryType;
};

export type State = ReturnType<typeof useRecoveryStart>;
export type RecoveryType = 'password' | 'device';
