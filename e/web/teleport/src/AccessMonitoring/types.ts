export enum ColumnType {
  Date = 'date',
  BigInt = 'bigint',
  VarChar = 'varchar',
  Array = 'array',
}

export enum ReportState {
  Ready = 'READY',
  Running = 'RUNNING',
  Failed = 'FAILED',
}

// Types

export interface QueryResult {
  rows: {
    data: string[];
  }[];
}

export interface Report {
  name: string;
  description: string;
  results: ReportResult[];
  lastUpdated: Date;
}

export interface ReportColumn {
  name: string;
  type: ColumnType;
}

export interface ReportResult {
  name: string;
  query: string;
  columns: ReportColumn[];
  data: string[][]; // data is all the rows with the columns in the same order as the columns array
  title: string;
  description: string;
}

export interface ReportOverview {
  description: string;
  name: string;
  queries: string[];
}

export interface TableColumn {
  name: string;
  type: string;
  desc: string;
}

export interface TableSchema {
  columns: TableColumn[];
  name: string;
}

// Responses

interface BackendReport {
  desc: string;
  name: string;
  queries: string[];
}

export type GetReportsResponse = BackendReport[];

interface AuditQueryResult {
  audit_query: {
    name: string;
    title: string;
    query: string;
    description: string;
  };
  result: {
    column_info: ReportColumn[];
    rows: {
      data: string[];
    }[];
  };
}

export interface GetReportStateResponse {
  status: ReportState;
}

export interface GetReportResponse {
  name: string;
  description: string;
  audit_query_results: AuditQueryResult[];
  updated_at: string;
}

export interface GetSchemaResponse {
  views: TableSchema[];
}

export interface GetQueryResultResponse {
  result: QueryResult;
}

export interface RunQueryResponse {
  result_id: string;
}
