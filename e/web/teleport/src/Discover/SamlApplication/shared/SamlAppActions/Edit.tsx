import { ButtonSecondary, Box, Indicator } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { Attempt } from 'shared/hooks/useAsync';
import { useTheme } from 'styled-components';
import { SamlMeta } from 'teleport/Discover/useDiscover';
import ErrorMessage from 'teleport/components/AgentErrorMessage';

import { DiscoverUpdate } from 'e-teleport/Discover';

import type { ResourceSpec } from 'teleport/Discover/SelectResource/types';

export function Edit({
  open,
  onClose,
  resourceSpec,
  agentMeta,
  attempt,
}: {
  open: boolean;
  onClose: () => void;
  resourceSpec: ResourceSpec;
  agentMeta: SamlMeta;
  attempt: Attempt<SamlMeta>;
}) {
  const theme = useTheme();

  function content() {
    switch (attempt.status) {
      case '':
      case 'processing':
        return (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        );
      case 'error':
        return <ErrorMessage message={attempt.statusText} />;
      case 'success':
        return (
          <DiscoverUpdate resourceSpec={resourceSpec} agentMeta={agentMeta} />
        );
    }
  }

  return (
    <Dialog
      dialogCss={() => ({
        height: '100%',
        width: '90%',
      })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={open}
    >
      <DialogHeader>
        <DialogTitle typography="body1" bold>
          Update SAML application
        </DialogTitle>
      </DialogHeader>

      <DialogContent
        maxHeight={'90%'}
        overflow={'auto'}
        bg={theme.colors.levels.sunken}
      >
        {content()}
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary disabled={false} onClick={onClose}>
          Close
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
