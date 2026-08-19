// Defines the type of subjects a access monitoring rule can operate on.
export enum AccessMonitoringRuleSubject {
  AccessRequest = 'access_request',
}

// Defines the access monitoring rule types.
export enum AccessMonitoringRuleType {
  Notification = 'notification',
  Review = 'review',
}

export interface AccessMonitoringRuleFilter {
  limit?: number;
  startKey?: string;
  subject?: AccessMonitoringRuleSubject;
}

type KindAccessMonitoringRule = 'access_monitoring_rule';

export enum AccessMonitoringRuleVersion {
  V1 = 'v1',
}

/**
 * This data structure will hold the exact response given back from a request.
 * May include fields with snake_case that does not conform to JS conventions
 * but we need to keep all fields as is since we need to send the response
 * back with whatever modifications the user made to fields for upsert actions.
 *
 * Only the fields the UI need to access is defined here
 * (there may be more accessible)
 */
export interface AccessMonitoringRule {
  kind: KindAccessMonitoringRule;
  version: AccessMonitoringRuleVersion;
  metadata: {
    name: string;
  };
  spec: {
    /**
     * subjects the rule operates on, can be a resource kind or a particular
     * resource property.
     */
    subjects: string[];
    /**
     * states are the desired state which the monitoring rule is attempting
     * to bring the subjects matching the condition to.
     *
     * Deprecated: use desired_state instead.
     */
    states?: string[];
    /**
     * condition is a predicate expression that operates on the specified
     * subject resources, and determines whether the subject will be moved
     * into desired state.
     */
    condition: string;
    /**
     * desired_state is the desired state which the monitoring rule is attempting
     * to bring the subjects matching the condition to.
     */
    desired_state?: string;
    /**
     * schedules specifies a map of named schedules that can be used to
     * configure time-based conditions.
     */
    schedules?: Record<string, AccessMonitoringRuleSchedule>;
    /**
     * notification defines the plugin configuration for notifications
     * if rule is triggered.
     */
    notification?: {
      /**
       * name is the name of the plugin to which this configuration
       * should apply.
       */
      name: string;
      /**
       * recipients is the list of recipients the plugin should notify.
       */
      recipients?: string[];
    };
    /**
     * automatic_review defines the configuration for automatic review rules.
     */
    automatic_review?: {
      /**
       * integration is the name of the integration/plugin to which this configuration
       * should apply. Set `builtin` if the rule should be monitored by Teleport.
       */
      integration: string;
      /**
       * decision specifies the proposed state of the access review submission.
       * This can be either `APPROVED` or `DENIED`.
       */
      decision: string;
    };
  };
}

export interface AccessMonitoringRuleSchedule {
  /**
   * time contains in-line schedule configuration.
   */
  time: {
    /**
     * timezone specifies the timezone for this schedule.
     * Accepted timezone values are defined in the IANA Time Zone Database,
     * such as "America/Los_Angeles", "Europe/Lisbon", or * "Asia/Singapore".
     */
    timezone: string;
    /**
     * shifts specifies a list of shifts that make up this schedule.
     */
    shifts: AccessMonitoringRuleScheduleShift[];
  };
}

interface AccessMonitoringRuleScheduleShift {
  /**
   * weekday specifies the weekday of the shift, e.g., "Monday", "Tuesday".
   */
  weekday: string;
  /**
   * start specifies the start time of the shift. This is a value between
   * "00:00" and "23:59".
   */
  start: string;
  /**
   * end specifies the end time of the shift. This is a value between
   * "00:00" and "23:59".
   */
  end: string;
}

export interface AccessMonitoringRuleWithYaml {
  object: AccessMonitoringRule;
  /**
   * yaml string used with yaml editors.
   */
  yaml: string;
}

export type AccessMonitoringRuleUpsertRequest =
  | UpsertAccessMonitoringRuleObject
  | UpsertAccessMonitoringRuleYaml;

type UpsertAccessMonitoringRuleObject = {
  object: AccessMonitoringRule;
};

type UpsertAccessMonitoringRuleYaml = {
  /**
   * yaml string used with yaml editors.
   */
  yaml: string;
};
