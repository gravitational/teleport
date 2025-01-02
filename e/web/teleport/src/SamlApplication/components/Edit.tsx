import { useTheme } from 'styled-components';

import { ButtonSecondary } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

export function Edit({
  open,
  onClose,
  Content,
}: {
  open: boolean;
  onClose: () => void;
  Content: () => React.ReactNode;
}) {
  const theme = useTheme();

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
        <DialogTitle>Update SAML application</DialogTitle>
      </DialogHeader>

      <DialogContent
        maxHeight={'90%'}
        overflow={'auto'}
        bg={theme.colors.levels.sunken}
      >
        <Content />
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary disabled={false} onClick={onClose}>
          Close
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
