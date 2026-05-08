import { format } from 'date-fns';
import { useLocation, useNavigate } from 'react-router';

import { Alert } from 'design/Alert';
import { DATE_FORMAT } from 'design/datetime/constants';
import { ArrowForward, Info, ListMagnifyingGlass } from 'design/Icon';

import {
  accessListRequiresReview,
  getReviewDate,
} from 'e-teleport/AccessListManagement/AccessListManagementContext';
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
  const navigate = useNavigate();

  const canReview = perms.isOwner || perms.adminWhoCanEdit;
  const requiresReview =
    !isReadOnlyOktaList &&
    accessListRequiresReview({
      todayDate: new Date(),
      reviewDate: getReviewDate(accessList),
    });

  if (!isReviewable(accessList.type) && location.hash === '#review') {
    return (
      <Alert kind="neutral" icon={Info}>
        Static Access Lists do not support reviews because they are managed
        outside the Web UI.
      </Alert>
    );
  }

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
            navigate(`${location.pathname}#review`, { state: location.state }),
        }}
      >
        This Access List requires review by{' '}
        {format(accessList.audit.nextDate, DATE_FORMAT)}.
      </Alert>
    );
  }

  return null;
};
