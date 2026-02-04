import { format } from 'date-fns';
import { useHistory, useLocation } from 'react-router';

import { Alert } from 'design/Alert';
import { DATE_FORMAT } from 'design/datetime/constants';
import { ArrowForward, Info, ListMagnifyingGlass } from 'design/Icon';

import { isReviewable } from 'e-teleport/services/accessmanagement';

import { AccessListModified, Perms } from './Shared';

export const ReviewBanner = ({
  accessList,
  perms,
  isReadOnlyOktaList = false,
}: {
  accessList: AccessListModified;
  perms: Perms;
  isReadOnlyOktaList?: boolean;
}) => {
  const location = useLocation();
  const history = useHistory();

  const canReview = perms.isOwner || perms.adminWhoCanEdit;
  const requiresReview =
    isReviewable(accessList.type) &&
    !isReadOnlyOktaList &&
    (accessList.requiresReview || accessList.audit.nextDate < new Date());

  if (!requiresReview && canReview && location.hash === '#review') {
    return (
      <Alert kind="neutral" icon={Info}>
        {isReadOnlyOktaList
          ? 'This Access List does not require review; it is managed by Okta and is read-only in Teleport.'
          : `This Access List does not require review until ${format(accessList.audit.nextDate, DATE_FORMAT)}.`}
      </Alert>
    );
  }

  if (requiresReview && canReview) {
    return (
      <Alert
        mb={0}
        kind="outline-info"
        icon={ListMagnifyingGlass}
        primaryAction={{
          content: (
            <>
              Start Review
              <ArrowForward size={18} ml={2} />
            </>
          ),
          onClick: () =>
            history.push(`${location.pathname}#review`, location.state),
        }}
      >
        This Access List requires review by{' '}
        {format(accessList.audit.nextDate, DATE_FORMAT)}.
      </Alert>
    );
  }

  return null;
};
