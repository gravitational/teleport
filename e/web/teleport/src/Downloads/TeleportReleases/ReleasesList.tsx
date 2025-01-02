import styled from 'styled-components';

import Table, { Cell } from 'design/DataTable';
import Link from 'design/Link';

import { Asset } from 'e-teleport/services/downloads';

type ReleasesListProps = {
  assets: Asset[];
  emptyText?: string;
};

export const ReleasesList = ({
  assets,
  emptyText = 'No Downloads Found',
}: ReleasesListProps) => {
  return (
    <StyledTable
      style={{
        borderRadius: '8px',
      }}
      data={assets}
      columns={[
        {
          key: 'description',
          headerText: 'Name',
          isSortable: true,
          render: ({ description }) => (
            <Cell style={{ padding: '28px 24px', fontWeight: '700' }}>
              {description}
            </Cell>
          ),
        },
        {
          key: 'displaySize',
          headerText: 'Size',
          isSortable: true,
          render: ({ displaySize }) => <Cell>{displaySize}</Cell>,
        },
        {
          key: 'url',
          headerText: 'DownloadLink',
          isSortable: true,
          render: ({ url, name }) => (
            <Cell style={{ fontSize: '14px' }}>
              <Link href={url}>{name}</Link>
            </Cell>
          ),
        },
      ]}
      emptyText={emptyText}
      isSearchable={false}
      disableFilter={true}
    />
  );
};

const StyledTable = styled(Table)`
  & > thead > tr > th {
    background: ${props => props.theme.colors.spotBackground[1]};
  }
  border-radius: 8px;
  overflow: hidden;
  box-shadow: ${props => props.theme.boxShadow[0]};
` as typeof Table;
