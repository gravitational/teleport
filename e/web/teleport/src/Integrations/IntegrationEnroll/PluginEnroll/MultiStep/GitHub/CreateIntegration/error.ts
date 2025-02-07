import { Attempt } from 'shared/hooks/useAsync';

import { ApiError } from 'teleport/services/api/parseError';

export function isAlreadyExistsError<T>(attempt: Attempt<T>) {
  return (
    attempt.status === 'error' &&
    attempt.error instanceof ApiError &&
    attempt.error.response.status === 409
  );
}
