import Alert from 'design/Alert';
import Box from 'design/Box';
import ButtonLink from 'design/ButtonLink';
import { Attempt } from 'shared/hooks/useAttemptNext';

export const FailedAttempt = ({
  attempt,
  retry,
}: {
  attempt: Attempt;
  retry?(): void;
}) => {
  return (
    <Alert kind="danger">
      {attempt.statusText}
      {retry && (
        <Box flex="0 0 auto" ml={2}>
          <ButtonLink onClick={() => retry()}>Retry</ButtonLink>
        </Box>
      )}
    </Alert>
  );
};
