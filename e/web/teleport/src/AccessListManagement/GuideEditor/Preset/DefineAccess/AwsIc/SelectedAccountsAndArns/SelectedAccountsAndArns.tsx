import { useState } from 'react';

import { Box, Flex } from 'design';
import InputSearch from 'design/DataTable/InputSearch';
import { ClientSidePager } from 'design/DataTable/Pager';
import paginateData from 'design/DataTable/Pager/paginateData';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { NoResultsFound } from 'e-teleport/AccessListManagement/GuideEditor/Shared';

import { AwsIcApp } from '../../../role/conditions';
import { SelectedTable } from './SelectedTable';

const pageSize = 10;

export function SelectedAccountsAndArns({
  selectedApps,
}: {
  selectedApps: AwsIcApp[];
}) {
  const { guideEditor } = useAccessListManagementContext();
  const { awsIcRoleState } = guideEditor;

  const [searchValue, setSearchValue] = useState('');
  const [currentPage, setCurrentPage] = useState(0);

  let filteredSelectedApps = [...selectedApps];
  if (searchValue) {
    const search = searchValue.toLocaleLowerCase();
    filteredSelectedApps = selectedApps.filter(app => {
      if (
        app.accountId.includes(search) ||
        app.friendlyAccountName?.toLocaleLowerCase().includes(search)
      ) {
        return true;
      }

      return app.arnMap
        .keys()
        .some(
          arn =>
            arn.toLocaleLowerCase().includes(search) ||
            app.arnMap.get(arn)?.toLocaleLowerCase().includes(search)
        );
    });
  }

  function nextPage() {
    setCurrentPage(currentPage + 1);
  }

  function prevPage() {
    setCurrentPage(currentPage - 1);
  }

  const paginatedApps = paginateData<AwsIcApp>(filteredSelectedApps, pageSize);

  return (
    <Box>
      <Flex gap={2}>
        <InputSearch
          searchValue={searchValue}
          setSearchValue={search => {
            setSearchValue(search);
            setCurrentPage(0);
          }}
          isDisabled={awsIcRoleState.fetchedApps.isError}
        />
        <ClientSidePager
          nextPage={nextPage}
          prevPage={prevPage}
          data={filteredSelectedApps}
          pageSize={pageSize}
          paginatedData={paginatedApps}
          currentPage={currentPage}
        />
      </Flex>

      <SelectedTable paginatedApps={paginatedApps} currentPage={currentPage} />

      {selectedApps.length > 0 && !filteredSelectedApps.length && (
        <Box m={3}>
          <NoResultsFound />
        </Box>
      )}
    </Box>
  );
}
