import { useCallback } from 'react';
import styled from 'styled-components';

import { Flex, Indicator } from 'design';
import { Danger } from 'design/Alert';
import Table, { Cell } from 'design/DataTable';
import { displayDate } from 'design/datetime';
import { Devices } from 'design/Icon';
import { MultiRowBox, Row } from 'design/MultiRowBox';
import {
  useInfiniteScroll,
  useKeyBasedPagination,
} from 'shared/hooks/useInfiniteScroll';

import { IconCell } from 'e-teleport/DeviceTrust/DeviceList/DeviceList';
import useTeleportE from 'e-teleport/useTeleportE';
import { Header } from 'teleport/Account/Header';
import { TrustedDevice } from 'teleport/DeviceTrust/types';

export const UserTrustedDevices = () => {
  const ctx = useTeleportE();

  const { fetch, resources, attempt } = useKeyBasedPagination({
    fetchFunc: useCallback(
      async (paginationParams, signal) => {
        return ctx.deviceService.fetchDevicesByUser(
          {
            startKey: paginationParams.startKey,
            limit: paginationParams.limit,
          },
          signal
        );
      },
      [ctx.deviceService]
    ),
    dataKey: 'items',
  });

  const { setTrigger } = useInfiniteScroll({
    fetch: fetch,
  });

  return (
    <MultiRowBox data-testid="user-trusted-devices">
      <Row>
        <Header
          title={
            <Flex gap={2} alignItems="center">
              Trusted Devices
            </Flex>
          }
          icon={<Devices />}
          description="Devices that have been authorized for your use with Teleport. Some actions may be disabled without a trusted device."
          actions={[]} // no actions, we're just displaying their devices
        />
      </Row>
      <Row>
        {attempt.status === 'failed' && (
          <Danger data-testid="user-trusted-devices-error">
            {attempt.statusText}
          </Danger>
        )}
        {attempt.status === 'processing' ? (
          <LoadingContainer>
            <Indicator />
          </LoadingContainer>
        ) : (
          <StyledTable
            data={resources}
            columns={[
              {
                key: 'osType',
                headerText: 'OS Type',
                render: ({ osType }) => <IconCell osType={osType} />,
              },
              {
                key: 'assetTag',
                headerText: 'Asset Tag',
              },
              {
                key: 'createTime',
                headerText: 'Date Created',
                render: ({ createTime }) => {
                  const validDate = createTime ? new Date(createTime) : '';
                  if (validDate) {
                    return <Cell>{displayDate(validDate)}</Cell>;
                  }
                  return null;
                },
              },
            ]}
            emptyText="No Devices Found"
          />
        )}
      </Row>
      <div ref={setTrigger} />
    </MultiRowBox>
  );
};

const LoadingContainer = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  padding: ${p => p.theme.space[5]}px 0;
  flex-direction: column;
`;

const StyledTable = styled(Table<TrustedDevice>)`
  & > tbody > tr > td,
  thead > tr > th {
    font-weight: 300;
    padding-bottom: ${props => props.theme.space[2]}px;
  }
`;
