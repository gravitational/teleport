import React from 'react';
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
    <Table
      style={{
        borderRadius: '4px',
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
