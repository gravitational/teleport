// Defines the type of subjects a access monitoring rule can operate on.
export enum AccessMonitoringRuleSubject {
  AccessRequest = 'access_request',
}

// Defines the access monitoring rule types.
export enum AccessMonitoringRuleType {
  Notification = 'notification',
  Approval = 'approval',
}

export interface AccessMonitoringRuleFilter {
  limit?: number;
  startKey?: string;
  subject?: AccessMonitoringRuleSubject;
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
     */
    states?: string[];
    /**
     * condition is a predicate expression that operates on the specified
     * subject resources, and determines whether the subject will be moved
     * into desired state.
     */
    condition: string;
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
  };
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

export interface AccessMonitoringRulePage {
  rules: AccessMonitoringRuleWithYaml[];
  startKey?: string;
}
