import Dialog, {
  DialogHeader,
  DialogContent,
  DialogTitle,
  DialogFooter,
} from 'design/Dialog';
import { ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import Text from 'design/Text';

import Label from 'design/Label';

interface ShowMoreGrantsDialogProps {
  roles: string[];
  traits: string[];
  onClose(): void;
}

export default function ShowMoreGrantsDialog({
  roles,
  traits,
  onClose,
}: ShowMoreGrantsDialogProps) {
  return (
    <Dialog
      open={true}
      onClose={onClose}
      disableEscapeKeyDown={false}
      disableBackdropClick={false}
    >
      <DialogHeader>
        <DialogTitle>Inherited Permissions</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Flex flexDirection="column" gap={2} maxWidth={500}>
          <Flex alignItems="start" gap={2}>
            <Text css={{ flexShrink: 0 }}>Roles:</Text>
            <Flex flexWrap="wrap" gap={1}>
              {roles.map(role => (
                <Label key={role} kind="secondary" css={{ fontSize: '12px' }}>
                  {role}
                </Label>
              ))}
            </Flex>
          </Flex>
          <Flex alignItems="start" gap={2}>
            <Text css={{ flexShrink: 0 }}>Traits:</Text>
            <Flex flexWrap="wrap" gap={1}>
              {traits.map(trait => (
                <Label key={trait} kind="secondary" css={{ fontSize: '12px' }}>
                  {trait}
                </Label>
              ))}
            </Flex>
          </Flex>
        </Flex>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
