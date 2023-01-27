import { SortType } from 'design/DataTable/types';

const ADVANCED_SEARCH_PARAM = 'query=';
const SIMPLE_SEARCH_PARAM = 'search=';
const SORT_SEARCH_PARAM = 'sort=';

export default function encodeUrlQueryParams(
  pathname: string,
  searchString: string,
  sort: SortType,
  isAdvancedSearch: boolean
) {
  if (!searchString && !sort) {
    return pathname;
  }
  const encodedQuery = encodeURIComponent(searchString);

  const searchParam = isAdvancedSearch
    ? ADVANCED_SEARCH_PARAM
    : SIMPLE_SEARCH_PARAM;

  if (encodedQuery && !sort) {
    return `${pathname}?${searchParam}${encodedQuery}`;
  }

  const sortParam = `${sort.fieldName}:${sort.dir.toLowerCase()}`;

  if (!encodedQuery && sort) {
    return `${pathname}?${`${SORT_SEARCH_PARAM}${sortParam}`}`;
  }

  return `${pathname}?${searchParam}${encodedQuery}&${`${SORT_SEARCH_PARAM}${sortParam}`}`;
}
