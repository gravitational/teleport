import { Link as InternalLink, useNavigate } from 'react-router';

import { Alert, Box, ButtonSecondary, ButtonWarning, P1, Text } from 'design';
import { OutlineWarn } from 'design/Alert/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import { P } from 'design/Text/Text';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'e-teleport/config';
import { accessManagementService } from 'e-teleport/services/accessmanagement';

import { useAccessListManagementContext } from '../AccessListManagementContext';

export function DeleteAccessListConfirmDialog({
  accessListName,
  accessListId,
  onClose,
  isOkta,
}: {
  accessListName: string;
  accessListId: string;
  isOkta: boolean;
  onClose(): void;
}) {
  const navigate = useNavigate();
  const { updateAccessListCache } = useAccessListManagementContext();

  const { attempt, setAttempt } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    // We don't need to setAttempt to "success"
    // since we are unmounting this after a successful
    // update.
    setAttempt({ status: 'processing' });
    accessManagementService
      .deleteAccessList(accessListId)
      .then(() => {
        // Because of backend caching, we send the deleted ID
        // as router state to be used to update the listing.
        updateAccessListCache({ mutationType: 'deleted', accessListId });
        navigate(cfg.getAccessListManagementRoute(), { replace: true });
      })
      .catch((e: Error) =>
        setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Delete Access List?</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <P1 mb={4}>
          Are you sure you want to delete{' '}
          <Text as="span" bold color="text.main">
            {accessListName}
          </Text>{' '}
          ?
        </P1>
        {isOkta && (
          <OutlineWarn linkColor="buttons.link.default">
            <Box>
              <P>
                This change will be reflected in Okta. All members from this
                Access List will also be unassigned from the targeted Okta group
                or application.
              </P>
              <P>
                To prevent Teleport from making modifications within Okta,
                ensure that the{' '}
                <InternalLink to={cfg.oss.routes.integrations}>
                  Okta integration
                </InternalLink>{' '}
                has been deleted.
              </P>
            </Box>
          </OutlineWarn>
        )}
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, Delete Access List
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
