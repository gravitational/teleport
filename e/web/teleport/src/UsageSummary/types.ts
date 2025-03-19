export type ProductUsage = {
  name: string;
  info: string;
  usages: Usage[];
  blurb?: string;
};

export type Usage = {
  name: string;
  total: number;
  percentageMax: number;
  hardMax: number;
  percentage: number;
};
