export interface ReportViewConfig {
  name: string;
  description: string;
  graphs: ReportGraphConfig[];
}

export enum GraphType {
  Bar,
}

export enum GroupMode {
  Grouped = 'grouped',
  Stacked = 'stacked',
}

interface BaseGraphConfig {
  type: GraphType;
  name: string;
}

export interface BarGraphConfig extends BaseGraphConfig {
  type: GraphType.Bar;
  name: string;
  indexByColumn: string;
  keysColumn: string;
  valueColumn: string;
  groupMode: GroupMode;
}

export type ReportGraphConfig = BarGraphConfig;

export const REPORT_VIEW_CONFIGS = new Map<string, ReportViewConfig>([
  [
    'privilege_access_report',
    {
      name: 'Privileged Access Report',
      description:
        'Identify sessions using weak security measures across your infrastructure',
      graphs: [
        {
          type: GraphType.Bar,
          name: 'database_sessions_with_weak_security',
          indexByColumn: 'event_date',
          keysColumn: 'user',
          valueColumn: 'count',
          groupMode: GroupMode.Stacked,
        },
        {
          type: GraphType.Bar,
          name: 'ssh_sessions_with_weak_security',
          indexByColumn: 'event_date',
          keysColumn: 'user',
          valueColumn: 'count',
          groupMode: GroupMode.Stacked,
        },
        {
          type: GraphType.Bar,
          name: 'kube_access_with_weak_security',
          indexByColumn: 'event_date',
          keysColumn: 'user',
          valueColumn: 'count',
          groupMode: GroupMode.Stacked,
        },
        {
          type: GraphType.Bar,
          name: 'db_postgres_user',
          indexByColumn: 'event_date',
          keysColumn: 'user',
          valueColumn: 'count',
          groupMode: GroupMode.Stacked,
        },
        {
          type: GraphType.Bar,
          name: 'kube_execs',
          indexByColumn: 'event_date',
          keysColumn: 'user',
          valueColumn: 'count',
          groupMode: GroupMode.Stacked,
        },
        {
          type: GraphType.Bar,
          name: 'cert_expiration_more_than_1d',
          indexByColumn: 'event_date',
          keysColumn: 'user',
          valueColumn: 'count',
          groupMode: GroupMode.Stacked,
        },
        {
          type: GraphType.Bar,
          name: 'instance_join_token_less_than_1d',
          indexByColumn: 'event_date',
          keysColumn: 'node_name',
          valueColumn: 'count',
          groupMode: GroupMode.Stacked,
        },
        {
          type: GraphType.Bar,
          name: 'session_start_root_user',
          indexByColumn: 'event_date',
          keysColumn: 'user',
          valueColumn: 'count',
          groupMode: GroupMode.Stacked,
        },
        {
          type: GraphType.Bar,
          name: 'kube_system_api_calls',
          indexByColumn: 'event_date',
          keysColumn: 'user',
          valueColumn: 'count',
          groupMode: GroupMode.Stacked,
        },
      ],
    },
  ],
]);
