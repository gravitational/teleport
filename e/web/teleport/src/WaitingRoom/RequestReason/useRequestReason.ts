import { useState } from 'react';
import useAttempt from 'shared/hooks/useAttempt';

export default function useRequestReason({ onCreateRequest, prompt }: Props) {
  const [attempt, attemptActions] = useAttempt({});
  const [reason, setReason] = useState('');

  function createRequest() {
    attemptActions.start();
    onCreateRequest(reason.trim()).catch(attemptActions.error);
  }

  return {
    attempt,
    reason,
    setReason,
    createRequest,
    prompt,
  };
}

export type Props = {
  onCreateRequest(reason?: string): Promise<any>;
  prompt?: string;
};
