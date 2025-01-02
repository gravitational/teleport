import { useHistory, useLocation } from 'react-router';

import cfg from 'e-teleport/config';
import { encodeUrlQueryParams } from 'teleport/components/hooks/useUrlFiltering';

export const useOnClickNestedList = () => {
  const history = useHistory();
  const location = useLocation<{
    previousPaths?: string[];
  }>();

  return (accessListId: string) => {
    const nextListPath = cfg.getAccessListManagementRoute(accessListId);
    const currentListPath = encodeUrlQueryParams({
      pathname: location.pathname,
    });
    const previousPaths = location.state?.previousPaths || [];

    if (previousPaths[previousPaths.length - 1] !== nextListPath) {
      previousPaths.push(currentListPath);
    }

    history.push(nextListPath, {
      previousPaths,
    });
  };
};
