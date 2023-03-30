// Device type as sent by web api.
export type TrustedDevice = {
  id: string;
  assetTag: string;
  osType: 'Windows' | 'Linux' | 'macOS';
  enrollStatus: string;
};

export type TrustedDeviceResponse = {
  items: TrustedDevice[];
  startKey: string;
};
