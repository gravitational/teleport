import { ButtonSecondary, Flex, Text } from 'design';
import Dialog, { DialogContent } from 'design/Dialog';

import { DefinableResourceAccessFields } from '../role/listaccess';

/**
 * Used to block user from going to the next step in a flow
 * and gives feedback to user to remove access to proceed.
 */
export function RemoveAccessDialog({
  onClose,
  field,
}: {
  onClose(): void;
  field: DefinableResourceAccessFields;
}) {
  let resourceName = '';
  switch (field) {
    case 'app_labels':
      resourceName = 'applications';
      break;
    case 'awsIc':
      resourceName = 'AWS IC applications';
      break;
    case 'db_labels':
      resourceName = 'databases';
      break;
    case 'github_permissions':
      resourceName = 'Git servers';
      break;
    case 'kubernetes_labels':
      resourceName = 'Kubernetes clusters';
      break;
    case 'node_labels':
      resourceName = 'servers';
      break;
    case 'windows_desktop_labels':
      resourceName = 'Windows desktops';
      break;
    case 'linux_desktop_labels':
      resourceName = 'Linux desktops';
      break;
    default:
      field satisfies never;
  }

  let filterKind = '';
  switch (field) {
    case 'app_labels':
    case 'db_labels':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels':
    case 'linux_desktop_labels':
      filterKind = 'labels';
      break;
    case 'awsIc':
      filterKind = 'AWS accounts';
      break;
    case 'github_permissions':
      filterKind = 'GitHub organizations';
      break;
    default:
      field satisfies never;
  }
  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '400px',
      })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogContent>
        <Text>
          No {resourceName} were found for the selected {filterKind}. Try a
          different selection or remove the selections.
        </Text>
      </DialogContent>
      <Flex gap={3}>
        <ButtonSecondary onClick={onClose} width="100%">
          Ok
        </ButtonSecondary>
      </Flex>
    </Dialog>
  );
}
