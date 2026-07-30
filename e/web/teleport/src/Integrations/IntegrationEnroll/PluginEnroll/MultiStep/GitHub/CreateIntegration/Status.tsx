import { PropsWithChildren, ReactNode } from 'react';

import { Mark, Text } from 'design';
import * as Icons from 'design/Icon';
import { Attempt } from 'shared/hooks/useAsync';

import { TextIcon } from 'teleport/Discover/Shared';

import { isAlreadyExistsError } from './error';

const Status: React.FC<
  PropsWithChildren<{ status: 'success' | 'error' | 'processing' }>
> = ({ children, status }) => {
  let Icon: ReactNode;
  switch (status) {
    case 'success':
      Icon = <Icons.Check size="medium" color="success.main" />;
      break;
    case 'error':
      Icon = <Icons.Warning size="medium" color="error.main" />;
      break;
    default:
      Icon = <Icons.Clock size="medium" />;
  }

  return (
    <TextIcon mb={3}>
      {Icon}
      <Text>{children}</Text>
    </TextIcon>
  );
};

export function AttemptStatus<T>({
  kind,
  resourceName,
  createAttempt,
  updateAttempt,
}: {
  kind: 'integration' | 'gitserver';
  resourceName: string;
  createAttempt: Attempt<T>;
  updateAttempt: Attempt<T>;
}) {
  const resource = kind === 'integration' ? 'GitHub integration' : 'Git server';

  if (
    updateAttempt.status === 'success' ||
    createAttempt.status === 'success'
  ) {
    const action = updateAttempt.status === 'success' ? 'Overwrote' : 'Created';
    return (
      <Status status="success">
        {action} {resource} named <Mark>{resourceName}</Mark>
      </Status>
    );
  }

  if (
    updateAttempt.status === 'processing' ||
    createAttempt.status === 'processing'
  ) {
    const action =
      updateAttempt.status === 'processing' ? 'Overwriting' : 'Creating';
    return (
      <Status status="processing">
        {action} {resource} named <Mark>{resourceName}</Mark>
      </Status>
    );
  }

  if (updateAttempt.status === 'error') {
    return (
      <Status status="error">
        Failed to overwrite GitHub integration: {updateAttempt.statusText}
      </Status>
    );
  }

  if (createAttempt.status === 'error') {
    let msg: React.ReactNode;
    let preMsg = '';

    if (isAlreadyExistsError(createAttempt)) {
      msg = (
        <>
          A {resource} with the name <Mark>{resourceName}</Mark> already exists
        </>
      );
    } else {
      msg = <>{createAttempt.statusText}</>;
      preMsg = `Failed to create ${resource}: `;
    }

    return (
      <Status status="error">
        <>
          {preMsg}
          {msg}
        </>
      </Status>
    );
  }
}
