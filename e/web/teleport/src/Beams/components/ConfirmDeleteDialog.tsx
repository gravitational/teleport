import {
  ButtonSecondary,
  ButtonWarning,
  Flex,
  P1,
  Spinner,
} from '@gravitational/design-system';
import styled from 'styled-components';

import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import { Beam } from 'e-teleport/services/beams/types';

import { resolveBeamName } from './constants';

// This is to cap the number of beams displayed in the delete dialog
// The rest are collapsed into a `and N more` message.
const MAX_DISPLAYED_BEAMS = 20;

export function ConfirmDeleteDialog({
  beams,
  isPending,
  onClose,
  onConfirm,
}: {
  beams: Beam[];
  isPending: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const single = beams.length === 1;
  const title = single ? 'Delete beam?' : `Delete ${beams.length} beams?`;
  const confirmLabel = single
    ? 'I understand, delete beam'
    : 'I understand, delete beams';
  return (
    <Dialog open onClose={isPending ? () => {} : onClose}>
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
      </DialogHeader>
      <DialogContent width="480px">
        {single ? (
          <>
            <P1 mb={3}>
              You are about to delete beam{' '}
              <strong>{resolveBeamName(beams[0])}</strong>. This will terminate
              everything running inside the beam.
            </P1>
            <P1 mb={0}>This cannot be undone.</P1>
          </>
        ) : beams.length === 2 ? (
          <>
            <P1 mb={3}>
              You are about to delete beams{' '}
              <strong>{resolveBeamName(beams[0])}</strong> and{' '}
              <strong>{resolveBeamName(beams[1])}</strong>. This will terminate
              everything running inside them.
            </P1>
            <P1 mb={0}>This cannot be undone.</P1>
          </>
        ) : (
          <>
            <P1 mb={2}>
              You are about to delete {beams.length} beams. This will terminate
              everything running inside them.
            </P1>
            <BeamQuote>{formatBeamList(beams)}</BeamQuote>
            <P1 mt={3} mb={0}>
              This cannot be undone.
            </P1>
          </>
        )}
      </DialogContent>
      <DialogFooter>
        <Flex gap={3}>
          <ButtonWarning disabled={isPending} onClick={onConfirm}>
            {isPending && <Spinner size="sm" color="text.muted" mr={2} />}
            {isPending ? 'Deleting...' : confirmLabel}
          </ButtonWarning>
          <ButtonSecondary disabled={isPending} onClick={onClose}>
            Cancel
          </ButtonSecondary>
        </Flex>
      </DialogFooter>
    </Dialog>
  );
}

function formatBeamList(beams: Beam[]): string {
  const names = beams.map(resolveBeamName);
  if (names.length <= MAX_DISPLAYED_BEAMS) {
    return names.join(', ');
  }
  const shown = names.slice(0, MAX_DISPLAYED_BEAMS).join(', ');
  const remaining = names.length - MAX_DISPLAYED_BEAMS;
  return `${shown}, and ${remaining} more`;
}

const BeamQuote = styled.blockquote`
  margin: 0;
  padding: ${({ theme }) => theme.space[2]}px ${({ theme }) => theme.space[3]}px;
  border-left: 3px solid
    ${({ theme }) => theme.colors.interactive.tonal.neutral[2]};
  background: ${({ theme }) => theme.colors.interactive.tonal.neutral[0]};
  border-radius: ${({ theme }) => theme.radii[2]}px;
  color: ${({ theme }) => theme.colors.text.slightlyMuted};
  font-size: ${({ theme }) => theme.fontSizes[1]}px;
  line-height: 1.5;
  word-break: break-word;
`;
