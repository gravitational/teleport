import {
  arrow,
  autoUpdate,
  offset,
  shift,
  size,
  useDismiss,
  useFloating,
  useInteractions,
} from '@floating-ui/react';
import { useState } from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import { Button } from 'design/Button';
import Flex from 'design/Flex';
import { ChevronRight } from 'design/Icon';
import Modal from 'design/Modal';
import { StyledPopover } from 'design/Popover';
import { Markdown } from 'shared/components/Markdown/Markdown';

interface SessionDescriptionProps {
  shortDescription: string;
  detailedDescription: string;
}

export function SessionDescription({
  shortDescription,
  detailedDescription,
}: SessionDescriptionProps) {
  const [arrowEl, setArrowEl] = useState<HTMLDivElement>(null);

  const [open, setOpen] = useState(false);

  const { context, floatingStyles, middlewareData, refs } = useFloating({
    open,
    onOpenChange: setOpen,
    middleware: [
      offset(8),
      shift({ padding: 8 }),
      size({
        apply({ availableWidth, availableHeight, elements }) {
          elements.floating.style.maxWidth = `${Math.min(650, availableWidth)}px`;
          elements.floating.style.maxHeight = `${Math.min(
            700,
            availableHeight
          )}px`;
        },
        padding: 8,
      }),
      arrow({
        element: arrowEl,
        padding: 8,
      }),
    ],
    placement: 'right',
    whileElementsMounted: autoUpdate,
  });

  const dismiss = useDismiss(context);

  const { getReferenceProps, getFloatingProps } = useInteractions([dismiss]);

  return (
    <>
      <Box
        backgroundColor="levels.surface"
        px="8px"
        pt={2}
        pb={1}
        mt={2}
        mb={3}
        borderRadius={3}
      >
        <Markdown text={shortDescription} />

        <Flex justifyContent="flex-end" mt={1} mr="-4px">
          <Button
            compact
            fill="minimal"
            intent="neutral"
            pr={1}
            pl={2}
            onClick={() => setOpen(true)}
            ref={refs.setReference}
            {...getReferenceProps()}
          >
            Show more <ChevronRight size="small" ml={1} />
          </Button>
        </Flex>
      </Box>

      {open && (
        <Modal open={true} BackdropProps={{ invisible: true }}>
          <StyledPopover
            shadow={true}
            ref={refs.setFloating}
            style={{ ...floatingStyles, overflow: 'visible' }}
            {...getFloatingProps()}
          >
            <Arrow
              ref={setArrowEl}
              style={{
                left: middlewareData.arrow?.x ?? '',
                top: middlewareData.arrow?.y ?? '',
              }}
            />

            <Box
              px={3}
              style={{ overflowY: 'auto' }}
              maxHeight="1200px"
              width="600px"
              data-scrollbar="default"
            >
              <Markdown text={detailedDescription} />
            </Box>
          </StyledPopover>
        </Modal>
      )}
    </>
  );
}

const Arrow = styled.div`
  position: absolute;
  width: 8px;
  left: -4px;
  height: 8px;
  top: 50%;
  background: ${p => p.theme.colors.levels.elevated};
  transform: rotate(-45deg);
  z-index: -1;
  border-left: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-top: 1px solid ${p => p.theme.colors.spotBackground[1]};
`;
