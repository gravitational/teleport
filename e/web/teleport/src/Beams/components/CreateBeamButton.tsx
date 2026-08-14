import { Button, Spinner } from '@gravitational/design-system';
import styled from 'styled-components';

import { Beam } from 'e-teleport/services/beams/types';

import { useCreateBeam } from './useBeamMutations';

export function CreateBeamButton({
  clusterId,
  onCreated,
}: {
  clusterId: string;
  onCreated: (beam: Beam) => void;
}) {
  const { mutate, isPending } = useCreateBeam(clusterId, {
    onSuccess: onCreated,
  });

  return (
    <StyledCreateButton disabled={isPending} onClick={() => mutate()}>
      {isPending && <Spinner size="sm" color="text.primaryInverse" mr={2} />}
      {isPending ? 'Creating...' : 'Create beam'}
    </StyledCreateButton>
  );
}

const StyledCreateButton = styled(Button).attrs({
  fill: 'filled' as const,
  intent: 'primary' as const,
})`
  &:disabled {
    background-color: ${({ theme }) =>
      theme.colors.interactive.solid.primary.default};
    color: ${({ theme }) => theme.colors.text.primaryInverse};
    opacity: 0.7;
  }
`;
