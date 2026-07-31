import {
  Button,
  ButtonSecondary,
  CaretDownIcon,
  CaretLeftIcon,
  CaretRightIcon,
} from '@gravitational/design-system';
import { useState } from 'react';
import styled from 'styled-components';

import Menu from 'design/Menu/Menu';
import MenuItem from 'design/Menu/MenuItem';
import { SortMenu } from 'shared/components/Controls/SortMenu';

import { Beam, BeamsSortField } from 'e-teleport/services/beams/types';

import {
  BEAM_SORT_ITEMS,
  coerceSortField,
  PAGE_SIZE,
  SortDir,
} from './constants';
import { BeamSelection } from './useBeamSelection';

type BeamsToolbarProps = {
  paging: {
    pageIndex: number | null;
    pageItemCount: number;
    hasPrevPage: boolean;
    hasNextPage: boolean;
    isFetching: boolean;
    onGoPrev: () => void;
    onGoNext: () => void;
  };
  sort: {
    field: BeamsSortField;
    dir: SortDir;
    onChange: (field: BeamsSortField, dir: SortDir) => void;
  };
  filter: {
    filterOwn: boolean;
    onToggle: () => void;
  };
  bulkDelete: {
    enabled: boolean;
    isPending: boolean;
    onRequest: (beams: Beam[]) => void;
  };
  selection: BeamSelection;
  noResults: boolean;
};

export function BeamsToolbar({
  paging,
  sort,
  filter,
  bulkDelete,
  selection,
  noResults,
}: BeamsToolbarProps) {
  const viewingText = formatViewingText(paging);

  return (
    <Toolbar>
      <ToolbarLeftControls>
        <Pager>
          {!noResults && <PagerLabel>{viewingText}</PagerLabel>}
          <PagerArrow
            aria-label="Previous page"
            disabled={!paging.hasPrevPage || paging.isFetching}
            onClick={paging.hasPrevPage ? paging.onGoPrev : undefined}
          >
            <CaretLeftIcon boxSize={4} />
          </PagerArrow>
          <PagerArrow
            aria-label="Next page"
            disabled={!paging.hasNextPage || paging.isFetching}
            onClick={paging.hasNextPage ? paging.onGoNext : undefined}
          >
            <CaretRightIcon boxSize={4} />
          </PagerArrow>
        </Pager>
        {bulkDelete.enabled && selection.size > 0 && (
          <Button
            fill="border"
            intent="danger"
            disabled={bulkDelete.isPending}
            onClick={() => bulkDelete.onRequest(selection.selected)}
          >
            Delete ({selection.size})
          </Button>
        )}
      </ToolbarLeftControls>
      <ToolbarRightControls>
        <CreatedByMenu
          filterOwn={filter.filterOwn}
          onToggle={filter.onToggle}
        />
        {!noResults && (
          <SortMenu
            selectedKey={sort.field}
            selectedOrder={sort.dir}
            onChange={(key, order) => {
              const field = coerceSortField(key);
              if (field) sort.onChange(field, order);
            }}
            items={BEAM_SORT_ITEMS}
          />
        )}
      </ToolbarRightControls>
    </Toolbar>
  );
}

function formatViewingText({
  pageIndex,
  pageItemCount,
  hasPrevPage,
  hasNextPage,
}: BeamsToolbarProps['paging']): string {
  if (pageIndex === null) {
    const plural = pageItemCount === 1 ? '' : 's';
    return `Viewing ${pageItemCount} result${plural} on this page`;
  }
  const start = pageItemCount > 0 ? pageIndex * PAGE_SIZE + 1 : 0;
  const end = pageIndex * PAGE_SIZE + pageItemCount;
  const isSinglePage = !hasPrevPage && !hasNextPage;
  return isSinglePage
    ? `Viewing ${start}-${end} of ${pageItemCount}`
    : `Viewing ${start}-${end}`;
}

const CREATED_BY_OPTIONS = [
  { filterOwn: false, label: 'Created by: Anyone' },
  { filterOwn: true, label: 'Created by: Me' },
] as const;

function labelForFilter(filterOwn: boolean): string {
  return CREATED_BY_OPTIONS.find(o => o.filterOwn === filterOwn).label;
}

function CreatedByMenu({
  filterOwn,
  onToggle,
}: {
  filterOwn: boolean;
  onToggle: () => void;
}) {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const close = () => setAnchorEl(null);

  return (
    <>
      <ButtonSecondary
        size="md"
        onClick={e => setAnchorEl(e.currentTarget)}
        aria-label="Created by filter"
      >
        {labelForFilter(filterOwn)}
        <CaretDownIcon boxSize={4} ml={2} />
      </ButtonSecondary>
      <Menu
        open={!!anchorEl}
        anchorEl={anchorEl}
        onClose={close}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        getContentAnchorEl={null}
      >
        {CREATED_BY_OPTIONS.map(option => (
          <MenuItem
            key={option.label}
            onClick={() => {
              if (option.filterOwn !== filterOwn) {
                onToggle();
              }
              close();
            }}
          >
            {option.label}
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}

const Toolbar = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space[3]}px;
  padding-bottom: ${({ theme }) => theme.space[4]}px;
`;

const ToolbarLeftControls = styled.div`
  display: inline-flex;
  align-items: center;
  gap: ${({ theme }) => theme.space[3]}px;
`;

const ToolbarRightControls = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space[3]}px;

  & > button {
    height: 32px;
    padding: 0 ${({ theme }) => theme.space[3]}px;
    background-color: ${({ theme }) =>
      theme.colors.interactive.tonal.neutral[0]};
    color: ${({ theme }) => theme.colors.text.slightlyMuted};
    border-color: transparent;

    &:hover,
    &:focus-visible {
      background-color: ${({ theme }) =>
        theme.colors.interactive.tonal.neutral[1]};
      border-color: transparent;
    }
  }
`;

const Pager = styled.div`
  display: inline-flex;
  align-items: center;
  gap: ${({ theme }) => theme.space[2]}px;
`;

const PagerLabel = styled.span`
  color: ${({ theme }) => theme.colors.text.slightlyMuted};
  ${({ theme }) => theme.typography.body2};
`;

const PagerArrow = styled.button`
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border: none;
  background: transparent;
  color: ${({ theme }) => theme.colors.text.slightlyMuted};
  cursor: pointer;

  &:hover:not(:disabled) {
    color: ${({ theme }) => theme.colors.text.main};
  }

  &:focus {
    outline: none;
  }

  &:focus-visible:not(:disabled) {
    outline: 2px solid ${({ theme }) => theme.colors.brand};
    outline-offset: 2px;
    border-radius: ${({ theme }) => theme.radii[1]}px;
  }

  &:disabled {
    color: ${({ theme }) => theme.colors.text.disabled};
    cursor: not-allowed;
    opacity: 0.5;
  }
`;
