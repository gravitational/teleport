import styled from 'styled-components';

import { ButtonPrimary } from 'design';
import { Plus } from 'design/Icon';
import { Indicator } from 'design/Indicator/Indicator';

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
      {isPending ? (
        <Indicator size={16} color="text.primaryInverse" delay="none" mr={2} />
      ) : (
        <Plus size="small" mr={2} />
      )}
      {isPending ? 'Creating...' : 'Create beam'}
    </StyledCreateButton>
  );
}

const StyledCreateButton = styled(ButtonPrimary)`
  &:disabled {
    background-color: ${({ theme }) =>
      theme.colors.interactive.solid.primary.default};
    color: ${({ theme }) => theme.colors.text.primaryInverse};
    opacity: 0.7;
  }
`;
