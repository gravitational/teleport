import { format } from 'date-fns';
import { useLocation, useNavigate } from 'react-router';

import { Alert } from 'design/Alert';
import { DATE_FORMAT } from 'design/datetime/constants';
import { ArrowForward, Info, ListMagnifyingGlass } from 'design/Icon';

import { AccessListModified, useAccessListReviewStatus } from './Shared';

export const ReviewBanner = ({
  accessList,
}: {
  accessList: AccessListModified;
}) => {
  const location = useLocation();
  const navigate = useNavigate();
  const { canReview, requiresReview, reason } =
    useAccessListReviewStatus(accessList);

  if (!canReview) return null;

  if (!requiresReview) {
    if (location.hash !== '#review') return null;

    const reasonMessage =
      reason === 'static'
        ? 'This Access List does not require review; it is Static and managed outside the Web UI.'
        : reason === 'okta-read-only'
          ? 'This Access List does not require review; it is managed by Okta and is read-only in Teleport.'
          : `This Access List does not require review until ${format(accessList.audit.nextDate, DATE_FORMAT)}.`;

    return (
      <Alert kind="neutral" icon={Info}>
        {reasonMessage}
      </Alert>
    );
  }

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
};
