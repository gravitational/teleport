export type Asset = {
  url: string;
  name: string;
  os: OS;
  sha256: string;
  displaySize: string;
  description: string;
  kind: Kind;
};

export type Release = {
  version: string;
  assets: Asset[];
};

export type Kind = 'Teleport' | 'tsh client' | 'Teleport Connect';

export type OS = 'Linux' | 'macOS' | 'Windows';
