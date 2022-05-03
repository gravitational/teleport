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

  if (encodedQuery && !sort) {
    return `${pathname}?${
      isAdvancedSearch ? ADVANCED_SEARCH_PARAM : SIMPLE_SEARCH_PARAM
    }${encodedQuery}`;
  }

  if (!encodedQuery && sort) {
    return `${pathname}?${`${SORT_SEARCH_PARAM}${
      sort.fieldName
    }:${sort.dir.toLowerCase()}`}`;
  }

  return `${pathname}?${
    isAdvancedSearch ? ADVANCED_SEARCH_PARAM : SIMPLE_SEARCH_PARAM
  }${encodedQuery}&${`${SORT_SEARCH_PARAM}${
    sort.fieldName
  }:${sort.dir.toLowerCase()}`}`;
}
