import { Alert } from 'design/Alert';
import { Attempt } from 'shared/hooks/useAttemptNext';

export const FailedAttempt = ({
  attempt,
  retry,
}: {
  attempt: Attempt;
  retry?(): void;
}) => {
  return (
    <Alert
      kind="danger"
      primaryAction={retry && { content: 'Retry', onClick: () => retry() }}
    >
      {attempt.statusText}
    </Alert>
  );
};
