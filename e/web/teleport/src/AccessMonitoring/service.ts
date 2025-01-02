import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

import {
  GetQueryResultResponse,
  GetReportResponse,
  GetReportsResponse,
  GetReportStateResponse,
  GetSchemaResponse,
  Report,
  ReportOverview,
  RunQueryResponse,
  TableSchema,
} from './types';

export async function getReports(clusterId: string): Promise<ReportOverview[]> {
  const res: GetReportsResponse = await api.get(
    cfg.getAccessMonitoringReportsUrl(clusterId)
  );

  return res.map(report => ({
    description: report.desc,
    name: report.name,
    queries: report.queries,
  }));
}

export async function getSchema(clusterId: string): Promise<TableSchema[]> {
  const res: GetSchemaResponse = await api.get(
    cfg.getAccessMonitoringSchemaUrl(clusterId)
  );

  return res.views;
}

export async function getReport(
  clusterId: string,
  reportName: string,
  days: number
): Promise<Report> {
  const res: GetReportResponse = await api.get(
    cfg.getAccessMonitoringReportUrl(clusterId, reportName, days)
  );

  const report: Report = {
    name: res.name,
    description: res.description,
    results: [],
    lastUpdated: new Date(res.updated_at),
  };

  for (const result of res.audit_query_results) {
    // remove the first row which is the column names
    const rows = result.result.rows.slice(1);

    report.results.push({
      query: result.audit_query.query,
      columns: result.result.column_info,
      data: rows.map(row => row.data),
      name: result.audit_query.name,
      title: result.audit_query.title,
      description: result.audit_query.description,
    });
  }

  return report;
}

export function runReport(
  clusterId: string,
  reportName: string,
  days: string
): Promise<Response> {
  return api.fetch(cfg.getAccessMonitoringReportRunUrl(clusterId, reportName), {
    method: 'POST',
    body: JSON.stringify({ days: parseInt(days, 10) }),
  });
}

export function getQueryResult(
  clusterId: string,
  resultId: string
): Promise<GetQueryResultResponse> {
  return api.post(cfg.getAccessMonitoringQueryResultUrl(clusterId), {
    result_id: resultId,
  });
}

export function runQuery(
  clusterId: string,
  query: string,
  timeframe: number
): Promise<RunQueryResponse> {
  return api.post(cfg.getAccessMonitoringQueryRunUrl(clusterId), {
    query,
    days: timeframe,
  });
}

export function getReportState(
  clusterId: string,
  reportName: string,
  timeframe: number
): Promise<GetReportStateResponse> {
  return api.get(
    cfg.getAccessMonitoringReportStateUrl(clusterId, reportName, timeframe)
  );
}
