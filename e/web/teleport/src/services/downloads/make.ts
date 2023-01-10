import type { Release, Asset, Kind, OS } from './types';

export const makeReleases = (json: any): Release[] => {
  if (!Array.isArray(json)) {
    return [];
  }

  return json.map((item): Release => {
    return {
      version: item.version || '',
      assets:
        item.assets?.map((assetItem): Asset => {
          const os = makeOS(assetItem.os);
          const kind = makeKind(assetItem.name, os);
          return {
            name: assetItem.name || '',
            os: os,
            kind: kind,
            displaySize: assetItem.display_size || '',
            description: makeDescription(
              assetItem.description || '',
              kind,
              os,
              assetItem.public_url || ''
            ),
            sha256: assetItem.sha256 || '',
            url: assetItem.public_url || '',
          };
        }) || [],
    };
  });
};

export const makeDescription = (
  description: string,
  kind: Kind,
  os: OS,
  url: string
): string => {
  // append the '64-bit DEB/RPM' to Teleport Connect on Linux, otherwhise
  // return original description
  if (kind === 'Teleport Connect' && os === 'Linux') {
    if (url.endsWith('.tar.gz')) {
      return description;
    }

    const split = url.split('.');
    const packageType = split.length > 0 ? split[split.length - 1] : '';
    const bits = url.includes('64') ? '64-bit' : '';

    return `${description} ${bits} ${packageType.toUpperCase()}`;
  }

  return description;
};

export const makeKind = (name: string, os: OS): Kind => {
  if (!name) {
    return 'Teleport';
  }
  const lowerName = name.toLowerCase();

  if (lowerName.includes('tsh-')) {
    return 'tsh client';
  }

  if (!lowerName.includes('connect') && os === 'Windows') {
    return 'tsh client';
  }

  if (
    lowerName.includes('teleport-connect') ||
    lowerName.includes('teleport connect')
  ) {
    return 'Teleport Connect';
  }
  return 'Teleport';
};

export const makeOS = (jsonOS: string): OS => {
  switch (jsonOS) {
    case 'linux':
      return 'Linux';
    case 'windows':
      return 'Windows';
    case 'macos':
    case 'darwin':
      return 'macOS';
    default:
      return 'Linux';
  }
};
