import { useEffect, useState } from 'react';

import { Alert, Box, ButtonSecondary, Flex, H2, Indicator, Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import { HoverTooltip } from 'design/Tooltip';

import {
  getReviewDayOfMonthOption,
  getReviewFrequencyOption,
} from 'e-teleport/AccessListManagement/Shared/Audit';
import { getFormattedDate } from 'e-teleport/AccessListManagement/Shared/date';
import {
  AccessList,
  AccessListMember,
  AccessListReview,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { useServerSidePagination } from 'teleport/components/hooks';

import { noEditAcessMsg } from '../errors';
import { AccessListModified, ButtonPencil } from '../Shared';
import { EditAudit } from './EditAudit';
import { ViewReview } from './ViewReview';

export function AuditAndReviews({
  updateAccessList,
  accessList,
  canEditSpecs,
  canListReviews,
}: {
  canListReviews: boolean;
  canEditSpecs: boolean;
  accessList: AccessListModified;
  updateAccessList: (
    newAccessList: AccessList,
    members?: AccessListMember[]
  ) => void;
}) {
  const [showEditAudit, setShowEditAudit] = useState(false);
  const [selectedReview, setSelectedReview] = useState<AccessListReview>();

  const frequency = getReviewFrequencyOption(
    accessList.audit.recurrence.frequency
  ).label;
  const dayOfMonth = getReviewDayOfMonthOption(
    accessList.audit.recurrence.dayOfMonth
  ).label;

  const serverSidePagination = useServerSidePagination<AccessListReview>({
    pageSize: 20,
    fetchFunc: async (_, params) => {
      const { reviews, startKey } = await accessManagementService.fetchReviews(
        accessList.id,
        { startKey: params.startKey, limit: 10 }
      );
      return { agents: reviews, startKey };
    },
    clusterId: '',
    params: {},
  });

  useEffect(() => {
    // init fetch
    if (canListReviews) {
      serverSidePagination.fetch();
    }
  }, []);

  return (
    <Box>
      <Box mb={2}>
        <Flex gap={2} alignItems="center">
          <Text bold>Review Frequency:</Text> {frequency}, {dayOfMonth}
          <HoverTooltip tipContent={!canEditSpecs ? noEditAcessMsg : undefined}>
            <ButtonPencil
              onClick={() => setShowEditAudit(true)}
              disabled={!canEditSpecs}
              dataTestId="btn-audit"
            />
          </HoverTooltip>
        </Flex>
        <Flex gap={2}>
          <Text bold>Next Review:</Text>{' '}
          {getFormattedDate(accessList.audit.nextDate)}
        </Flex>
      </Box>

      {canListReviews &&
        serverSidePagination.attempt.status === 'processing' && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}
      {serverSidePagination.attempt.status === 'failed' && (
        <>
          <H2 bold mt={5} mb={3}>
            Past Audits:
          </H2>
          <Alert
            kind="danger"
            primaryAction={{
              content: 'Retry',
              onClick: () => serverSidePagination.fetch(),
            }}
          >
            {serverSidePagination.attempt.statusText}
          </Alert>
        </>
      )}
      {serverSidePagination.attempt.status === 'success' && (
        <>
          <H2 bold mt={5} mb={-3}>
            Past Audits:
          </H2>
          <Table
            data={serverSidePagination.fetchedData.agents}
            fetching={{
              fetchStatus: serverSidePagination.fetchStatus,
              onFetchNext: serverSidePagination.fetchNext,
              onFetchPrev: serverSidePagination.fetchPrev,
            }}
            serversideProps={{
              sort: undefined,
              setSort: () => undefined,
              serversideSearchPanel: undefined,
            }}
            columns={[
              {
                key: 'reviewDate',
                headerText: 'Audit Date',
                render: review => (
                  <Cell>{getFormattedDate(review.reviewDate)}</Cell>
                ),
              },
              {
                key: 'reviewers',
                headerText: 'Reviewers',
                render: review => <Cell>{review.reviewers.join(', ')}</Cell>,
              },
              {
                key: 'notes',
                headerText: 'Notes',
              },
              {
                altKey: 'options-btn',
                render: review => (
                  <Cell align="right">
                    <Flex alignItems="center" justifyContent="flex-end">
                      <ButtonSecondary
                        textTransform="none"
                        onClick={() => setSelectedReview(review)}
                        size="small"
                        width="140px"
                      >
                        View Changes
                      </ButtonSecondary>
                    </Flex>
                  </Cell>
                ),
              },
            ]}
            emptyText="No Audits Found"
          />
        </>
      )}

      {showEditAudit && (
        <EditAudit
          onClose={() => setShowEditAudit(false)}
          updateAccessList={updateAccessList}
          accessList={accessList}
        />
      )}
      {selectedReview && (
        <ViewReview
          onClose={() => setSelectedReview(null)}
          review={selectedReview}
        />
      )}
    </Box>
  );
}
