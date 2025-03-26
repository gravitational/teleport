export type ProductUsage = {
  name: string;
  info: string;
  blurb?: string;
  enabled: boolean;
  ctaUrl?: string;
  usages: Usage[];
};

export type Usage = {
  name: string;
  total: number;
  percentageMax: number;
  hardMax: number;
  percentage: number;
};
