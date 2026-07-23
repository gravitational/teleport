import { Location, useLocation, useNavigate } from 'react-router';

import { BeamsSortField } from 'e-teleport/services/beams/types';

import { coerceSortField, SortDir } from './constants';

const DEFAULT_SORT_FIELD: BeamsSortField = 'expires';
const DEFAULT_SORT_DIR: SortDir = 'ASC';

type PagingState = { prevPageTokens?: string[] };

export function useBeamsListParams() {
  const navigate = useNavigate();
  const location = useLocation() as Location<PagingState>;

  const params = new URLSearchParams(location.search);
  const pageToken = params.get('page') ?? '';
  const sortField =
    coerceSortField(params.get('sort_field') ?? '') ?? DEFAULT_SORT_FIELD;
  const sortDir: SortDir =
    params.get('sort_dir')?.toUpperCase() === 'DESC' ? 'DESC' : 'ASC';
  const filterOwn = params.get('own') !== 'false';
  const prevPageTokens = location.state?.prevPageTokens ?? [];

  function goNext(nextPageToken: string) {
    const next = new URLSearchParams(location.search);
    next.set('page', nextPageToken);
    navigate(
      { pathname: location.pathname, search: next.toString() },
      {
        replace: true,
        state: { prevPageTokens: [...prevPageTokens, pageToken] },
      }
    );
  }

  function goPrev() {
    const stack = [...prevPageTokens];
    const token = stack.pop();
    const next = new URLSearchParams(location.search);
    if (token) {
      next.set('page', token);
    } else {
      next.delete('page');
    }
    navigate(
      { pathname: location.pathname, search: next.toString() },
      { replace: true, state: { prevPageTokens: stack } }
    );
  }

  function setSort(field: BeamsSortField, dir: SortDir) {
    const next = new URLSearchParams(location.search);
    if (field === DEFAULT_SORT_FIELD) {
      next.delete('sort_field');
    } else {
      next.set('sort_field', field);
    }
    if (dir === DEFAULT_SORT_DIR) {
      next.delete('sort_dir');
    } else {
      next.set('sort_dir', dir);
    }
    next.delete('page');
    navigate(
      { pathname: location.pathname, search: next.toString() },
      { replace: true }
    );
  }

  function toggleFilterOwn() {
    const next = new URLSearchParams(location.search);
    if (filterOwn) {
      next.set('own', 'false');
    } else {
      next.delete('own');
    }
    next.delete('page');
    navigate(
      { pathname: location.pathname, search: next.toString() },
      { replace: true }
    );
  }

  const pageIndex =
    pageToken && prevPageTokens.length === 0 ? null : prevPageTokens.length;

  return {
    pageToken,
    sortField,
    sortDir,
    filterOwn,
    hasPrevPage: !!pageToken,
    pageIndex,
    goNext,
    goPrev,
    setSort,
    toggleFilterOwn,
  };
}
