import cfg from 'e-teleport/config';

interface RedirectConfigItem {
  primaryButtonText: string;
  url: string;
}

type RedirectConfig = Record<string, RedirectConfigItem>;

const redirectConfig: RedirectConfig = {
  tag: {
    url: cfg.routes.accessGraph.dashboard,
    primaryButtonText: 'Return to Access Graph',
  },
};

export function getSuccessPrimaryButtonState(search: string) {
  const params = new URLSearchParams(search);

  const key = params.get('redirect');

  if (!key || !redirectConfig[key]) {
    return {
      successPrimaryButtonText: null,
      successPrimaryButtonUrl: null,
    };
  }

  return {
    successPrimaryButtonUrl: redirectConfig[key].url,
    successPrimaryButtonText: redirectConfig[key].primaryButtonText,
  };
}
