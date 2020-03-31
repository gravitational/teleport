export type LicenseStatus = {
  html: string;
  text: string;
  type: string;
  severity: Severity;
};

export type Severity = 'info' | 'error' | 'warning';
