import { Link as InternalLink } from 'react-router-dom';

import { Alert, Box, ButtonSecondary, ButtonWarning, P1, Text } from 'design';
import { Warning } from 'design/Alert/Alert';
import { DialogContent, DialogFooter } from 'design/DialogConfirmation';
import { P } from 'design/Text/Text';
import { Attempt } from 'shared/hooks/useAttemptNext';

import cfg from 'e-teleport/config';
import { AccessListOrigin } from 'e-teleport/services/accessmanagement';

import { AccessListModified } from '../Shared';

export function DeleteAccessList({
  onDelete,
  onCancel,
  attempt,
  accessList,
  fetchRolesError,
}: {
  onDelete(): void;
  onCancel(): void;
  attempt: Attempt;
  accessList: AccessListModified;
  fetchRolesError?: string;
}) {
  const isOkta = accessList.origin === AccessListOrigin.Okta;
  const accessListTitle = accessList.title;

  return (
    <>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert>{attempt.statusText}</Alert>}
        {fetchRolesError && <Alert>{fetchRolesError}</Alert>}
        <P1 mb={4}>
          Are you sure you want to delete{' '}
          <Text as="span" bold color="text.main">
            {accessListTitle}
          </Text>
          ?
        </P1>
        {isOkta && (
          <Warning linkColor="buttons.link.default">
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
          </Warning>
        )}
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          mr="3"
          disabled={attempt.status === 'processing'}
          onClick={onDelete}
        >
          Yes, Delete Access List
        </ButtonWarning>
        <ButtonSecondary
          disabled={attempt.status === 'processing'}
          onClick={onCancel}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </>
  );
}
