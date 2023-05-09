// Device type as sent by web api.
export type TrustedDevice = {
  id: string;
  assetTag: string;
  osType: TrustedDeviceOSType;
  enrollStatus: string;
};

export type TrustedDeviceOSType = 'Windows' | 'Linux' | 'macOS';

export type TrustedDeviceResponse = {
  items: TrustedDevice[];
  startKey: string;
};
