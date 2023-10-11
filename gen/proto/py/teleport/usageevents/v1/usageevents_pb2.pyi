from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DiscoverResource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    DISCOVER_RESOURCE_UNSPECIFIED: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_SERVER: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_KUBERNETES: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_POSTGRES_SELF_HOSTED: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_MYSQL_SELF_HOSTED: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_MONGODB_SELF_HOSTED: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_POSTGRES_RDS: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_MYSQL_RDS: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_APPLICATION_HTTP: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_APPLICATION_TCP: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_WINDOWS_DESKTOP: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_SQLSERVER_RDS: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_POSTGRES_REDSHIFT: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_SQLSERVER_SELF_HOSTED: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_REDIS_SELF_HOSTED: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_POSTGRES_GCP: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_MYSQL_GCP: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_SQLSERVER_GCP: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_POSTGRES_REDSHIFT_SERVERLESS: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_POSTGRES_AZURE: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_DYNAMODB: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_CASSANDRA_KEYSPACES: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_CASSANDRA_SELF_HOSTED: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_ELASTICSEARCH_SELF_HOSTED: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_REDIS_ELASTICACHE: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_REDIS_MEMORYDB: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_REDIS_AZURE_CACHE: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_REDIS_CLUSTER_SELF_HOSTED: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_MYSQL_AZURE: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_SQLSERVER_AZURE: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_SQLSERVER_MICROSOFT: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_COCKROACHDB_SELF_HOSTED: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_MONGODB_ATLAS: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DATABASE_SNOWFLAKE: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DOC_DATABASE_RDS_PROXY: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DOC_DATABASE_HIGH_AVAILABILITY: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_DOC_DATABASE_DYNAMIC_REGISTRATION: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_SAML_APPLICATION: _ClassVar[DiscoverResource]
    DISCOVER_RESOURCE_EC2_INSTANCE: _ClassVar[DiscoverResource]

class DiscoverStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    DISCOVER_STATUS_UNSPECIFIED: _ClassVar[DiscoverStatus]
    DISCOVER_STATUS_SUCCESS: _ClassVar[DiscoverStatus]
    DISCOVER_STATUS_SKIPPED: _ClassVar[DiscoverStatus]
    DISCOVER_STATUS_ERROR: _ClassVar[DiscoverStatus]
    DISCOVER_STATUS_ABORTED: _ClassVar[DiscoverStatus]

class CTA(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    CTA_UNSPECIFIED: _ClassVar[CTA]
    CTA_AUTH_CONNECTOR: _ClassVar[CTA]
    CTA_ACTIVE_SESSIONS: _ClassVar[CTA]
    CTA_ACCESS_REQUESTS: _ClassVar[CTA]
    CTA_PREMIUM_SUPPORT: _ClassVar[CTA]
    CTA_TRUSTED_DEVICES: _ClassVar[CTA]
    CTA_UPGRADE_BANNER: _ClassVar[CTA]
    CTA_BILLING_SUMMARY: _ClassVar[CTA]

class IntegrationEnrollKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    INTEGRATION_ENROLL_KIND_UNSPECIFIED: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_SLACK: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_AWS_OIDC: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_PAGERDUTY: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_EMAIL: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_JIRA: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_DISCORD: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_MATTERMOST: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_MS_TEAMS: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_OPSGENIE: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_OKTA: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_JAMF: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_MACHINE_ID: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_MACHINE_ID_GITHUB_ACTIONS: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_MACHINE_ID_CIRCLECI: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_MACHINE_ID_GITLAB: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_MACHINE_ID_JENKINS: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_MACHINE_ID_ANSIBLE: _ClassVar[IntegrationEnrollKind]
    INTEGRATION_ENROLL_KIND_SERVICENOW: _ClassVar[IntegrationEnrollKind]

class Feature(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    FEATURE_UNSPECIFIED: _ClassVar[Feature]
    FEATURE_TRUSTED_DEVICES: _ClassVar[Feature]

class FeatureRecommendationStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = []
    FEATURE_RECOMMENDATION_STATUS_UNSPECIFIED: _ClassVar[FeatureRecommendationStatus]
    FEATURE_RECOMMENDATION_STATUS_NOTIFIED: _ClassVar[FeatureRecommendationStatus]
    FEATURE_RECOMMENDATION_STATUS_DONE: _ClassVar[FeatureRecommendationStatus]
DISCOVER_RESOURCE_UNSPECIFIED: DiscoverResource
DISCOVER_RESOURCE_SERVER: DiscoverResource
DISCOVER_RESOURCE_KUBERNETES: DiscoverResource
DISCOVER_RESOURCE_DATABASE_POSTGRES_SELF_HOSTED: DiscoverResource
DISCOVER_RESOURCE_DATABASE_MYSQL_SELF_HOSTED: DiscoverResource
DISCOVER_RESOURCE_DATABASE_MONGODB_SELF_HOSTED: DiscoverResource
DISCOVER_RESOURCE_DATABASE_POSTGRES_RDS: DiscoverResource
DISCOVER_RESOURCE_DATABASE_MYSQL_RDS: DiscoverResource
DISCOVER_RESOURCE_APPLICATION_HTTP: DiscoverResource
DISCOVER_RESOURCE_APPLICATION_TCP: DiscoverResource
DISCOVER_RESOURCE_WINDOWS_DESKTOP: DiscoverResource
DISCOVER_RESOURCE_DATABASE_SQLSERVER_RDS: DiscoverResource
DISCOVER_RESOURCE_DATABASE_POSTGRES_REDSHIFT: DiscoverResource
DISCOVER_RESOURCE_DATABASE_SQLSERVER_SELF_HOSTED: DiscoverResource
DISCOVER_RESOURCE_DATABASE_REDIS_SELF_HOSTED: DiscoverResource
DISCOVER_RESOURCE_DATABASE_POSTGRES_GCP: DiscoverResource
DISCOVER_RESOURCE_DATABASE_MYSQL_GCP: DiscoverResource
DISCOVER_RESOURCE_DATABASE_SQLSERVER_GCP: DiscoverResource
DISCOVER_RESOURCE_DATABASE_POSTGRES_REDSHIFT_SERVERLESS: DiscoverResource
DISCOVER_RESOURCE_DATABASE_POSTGRES_AZURE: DiscoverResource
DISCOVER_RESOURCE_DATABASE_DYNAMODB: DiscoverResource
DISCOVER_RESOURCE_DATABASE_CASSANDRA_KEYSPACES: DiscoverResource
DISCOVER_RESOURCE_DATABASE_CASSANDRA_SELF_HOSTED: DiscoverResource
DISCOVER_RESOURCE_DATABASE_ELASTICSEARCH_SELF_HOSTED: DiscoverResource
DISCOVER_RESOURCE_DATABASE_REDIS_ELASTICACHE: DiscoverResource
DISCOVER_RESOURCE_DATABASE_REDIS_MEMORYDB: DiscoverResource
DISCOVER_RESOURCE_DATABASE_REDIS_AZURE_CACHE: DiscoverResource
DISCOVER_RESOURCE_DATABASE_REDIS_CLUSTER_SELF_HOSTED: DiscoverResource
DISCOVER_RESOURCE_DATABASE_MYSQL_AZURE: DiscoverResource
DISCOVER_RESOURCE_DATABASE_SQLSERVER_AZURE: DiscoverResource
DISCOVER_RESOURCE_DATABASE_SQLSERVER_MICROSOFT: DiscoverResource
DISCOVER_RESOURCE_DATABASE_COCKROACHDB_SELF_HOSTED: DiscoverResource
DISCOVER_RESOURCE_DATABASE_MONGODB_ATLAS: DiscoverResource
DISCOVER_RESOURCE_DATABASE_SNOWFLAKE: DiscoverResource
DISCOVER_RESOURCE_DOC_DATABASE_RDS_PROXY: DiscoverResource
DISCOVER_RESOURCE_DOC_DATABASE_HIGH_AVAILABILITY: DiscoverResource
DISCOVER_RESOURCE_DOC_DATABASE_DYNAMIC_REGISTRATION: DiscoverResource
DISCOVER_RESOURCE_SAML_APPLICATION: DiscoverResource
DISCOVER_RESOURCE_EC2_INSTANCE: DiscoverResource
DISCOVER_STATUS_UNSPECIFIED: DiscoverStatus
DISCOVER_STATUS_SUCCESS: DiscoverStatus
DISCOVER_STATUS_SKIPPED: DiscoverStatus
DISCOVER_STATUS_ERROR: DiscoverStatus
DISCOVER_STATUS_ABORTED: DiscoverStatus
CTA_UNSPECIFIED: CTA
CTA_AUTH_CONNECTOR: CTA
CTA_ACTIVE_SESSIONS: CTA
CTA_ACCESS_REQUESTS: CTA
CTA_PREMIUM_SUPPORT: CTA
CTA_TRUSTED_DEVICES: CTA
CTA_UPGRADE_BANNER: CTA
CTA_BILLING_SUMMARY: CTA
INTEGRATION_ENROLL_KIND_UNSPECIFIED: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_SLACK: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_AWS_OIDC: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_PAGERDUTY: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_EMAIL: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_JIRA: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_DISCORD: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_MATTERMOST: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_MS_TEAMS: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_OPSGENIE: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_OKTA: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_JAMF: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_MACHINE_ID: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_MACHINE_ID_GITHUB_ACTIONS: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_MACHINE_ID_CIRCLECI: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_MACHINE_ID_GITLAB: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_MACHINE_ID_JENKINS: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_MACHINE_ID_ANSIBLE: IntegrationEnrollKind
INTEGRATION_ENROLL_KIND_SERVICENOW: IntegrationEnrollKind
FEATURE_UNSPECIFIED: Feature
FEATURE_TRUSTED_DEVICES: Feature
FEATURE_RECOMMENDATION_STATUS_UNSPECIFIED: FeatureRecommendationStatus
FEATURE_RECOMMENDATION_STATUS_NOTIFIED: FeatureRecommendationStatus
FEATURE_RECOMMENDATION_STATUS_DONE: FeatureRecommendationStatus

class UIBannerClickEvent(_message.Message):
    __slots__ = ["alert"]
    ALERT_FIELD_NUMBER: _ClassVar[int]
    alert: str
    def __init__(self, alert: _Optional[str] = ...) -> None: ...

class UIOnboardCompleteGoToDashboardClickEvent(_message.Message):
    __slots__ = ["username"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    username: str
    def __init__(self, username: _Optional[str] = ...) -> None: ...

class UIOnboardAddFirstResourceClickEvent(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class UIOnboardAddFirstResourceLaterClickEvent(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class UIOnboardSetCredentialSubmitEvent(_message.Message):
    __slots__ = ["username"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    username: str
    def __init__(self, username: _Optional[str] = ...) -> None: ...

class UIOnboardQuestionnaireSubmitEvent(_message.Message):
    __slots__ = ["username"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    username: str
    def __init__(self, username: _Optional[str] = ...) -> None: ...

class UIOnboardRegisterChallengeSubmitEvent(_message.Message):
    __slots__ = ["username", "mfa_type", "login_flow"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    MFA_TYPE_FIELD_NUMBER: _ClassVar[int]
    LOGIN_FLOW_FIELD_NUMBER: _ClassVar[int]
    username: str
    mfa_type: str
    login_flow: str
    def __init__(self, username: _Optional[str] = ..., mfa_type: _Optional[str] = ..., login_flow: _Optional[str] = ...) -> None: ...

class UIRecoveryCodesContinueClickEvent(_message.Message):
    __slots__ = ["username"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    username: str
    def __init__(self, username: _Optional[str] = ...) -> None: ...

class UIRecoveryCodesCopyClickEvent(_message.Message):
    __slots__ = ["username"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    username: str
    def __init__(self, username: _Optional[str] = ...) -> None: ...

class UIRecoveryCodesPrintClickEvent(_message.Message):
    __slots__ = ["username"]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    username: str
    def __init__(self, username: _Optional[str] = ...) -> None: ...

class DiscoverMetadata(_message.Message):
    __slots__ = ["id"]
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DiscoverResourceMetadata(_message.Message):
    __slots__ = ["resource"]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    resource: DiscoverResource
    def __init__(self, resource: _Optional[_Union[DiscoverResource, str]] = ...) -> None: ...

class DiscoverStepStatus(_message.Message):
    __slots__ = ["status", "error"]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    status: DiscoverStatus
    error: str
    def __init__(self, status: _Optional[_Union[DiscoverStatus, str]] = ..., error: _Optional[str] = ...) -> None: ...

class UIDiscoverStartedEvent(_message.Message):
    __slots__ = ["metadata", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverResourceSelectionEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverIntegrationAWSOIDCConnectEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverDatabaseRDSEnrollEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status", "selected_resources_count"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    SELECTED_RESOURCES_COUNT_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    selected_resources_count: int
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ..., selected_resources_count: _Optional[int] = ...) -> None: ...

class UICallToActionClickEvent(_message.Message):
    __slots__ = ["cta"]
    CTA_FIELD_NUMBER: _ClassVar[int]
    cta: CTA
    def __init__(self, cta: _Optional[_Union[CTA, str]] = ...) -> None: ...

class UIDiscoverDeployServiceEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status", "deploy_method", "deploy_type"]
    class DeployMethod(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        DEPLOY_METHOD_UNSPECIFIED: _ClassVar[UIDiscoverDeployServiceEvent.DeployMethod]
        DEPLOY_METHOD_AUTO: _ClassVar[UIDiscoverDeployServiceEvent.DeployMethod]
        DEPLOY_METHOD_MANUAL: _ClassVar[UIDiscoverDeployServiceEvent.DeployMethod]
    DEPLOY_METHOD_UNSPECIFIED: UIDiscoverDeployServiceEvent.DeployMethod
    DEPLOY_METHOD_AUTO: UIDiscoverDeployServiceEvent.DeployMethod
    DEPLOY_METHOD_MANUAL: UIDiscoverDeployServiceEvent.DeployMethod
    class DeployType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = []
        DEPLOY_TYPE_UNSPECIFIED: _ClassVar[UIDiscoverDeployServiceEvent.DeployType]
        DEPLOY_TYPE_INSTALL_SCRIPT: _ClassVar[UIDiscoverDeployServiceEvent.DeployType]
        DEPLOY_TYPE_AMAZON_ECS: _ClassVar[UIDiscoverDeployServiceEvent.DeployType]
    DEPLOY_TYPE_UNSPECIFIED: UIDiscoverDeployServiceEvent.DeployType
    DEPLOY_TYPE_INSTALL_SCRIPT: UIDiscoverDeployServiceEvent.DeployType
    DEPLOY_TYPE_AMAZON_ECS: UIDiscoverDeployServiceEvent.DeployType
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    DEPLOY_METHOD_FIELD_NUMBER: _ClassVar[int]
    DEPLOY_TYPE_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    deploy_method: UIDiscoverDeployServiceEvent.DeployMethod
    deploy_type: UIDiscoverDeployServiceEvent.DeployType
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ..., deploy_method: _Optional[_Union[UIDiscoverDeployServiceEvent.DeployMethod, str]] = ..., deploy_type: _Optional[_Union[UIDiscoverDeployServiceEvent.DeployType, str]] = ...) -> None: ...

class UIDiscoverDatabaseRegisterEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverDatabaseConfigureMTLSEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverDesktopActiveDirectoryToolsInstallEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverDesktopActiveDirectoryConfigureEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverAutoDiscoveredResourcesEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status", "resources_count"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    RESOURCES_COUNT_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    resources_count: int
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ..., resources_count: _Optional[int] = ...) -> None: ...

class UIDiscoverEC2InstanceSelectionEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverDeployEICEEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverCreateNodeEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverDatabaseConfigureIAMPolicyEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverPrincipalsConfigureEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverTestConnectionEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UIDiscoverCompletedEvent(_message.Message):
    __slots__ = ["metadata", "resource", "status"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    metadata: DiscoverMetadata
    resource: DiscoverResourceMetadata
    status: DiscoverStepStatus
    def __init__(self, metadata: _Optional[_Union[DiscoverMetadata, _Mapping]] = ..., resource: _Optional[_Union[DiscoverResourceMetadata, _Mapping]] = ..., status: _Optional[_Union[DiscoverStepStatus, _Mapping]] = ...) -> None: ...

class UICreateNewRoleClickEvent(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class UICreateNewRoleSaveClickEvent(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class UICreateNewRoleCancelClickEvent(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class UICreateNewRoleViewDocumentationClickEvent(_message.Message):
    __slots__ = []
    def __init__(self) -> None: ...

class AssistCompletionEvent(_message.Message):
    __slots__ = ["conversation_id", "total_tokens", "prompt_tokens", "completion_tokens"]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    TOTAL_TOKENS_FIELD_NUMBER: _ClassVar[int]
    PROMPT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COMPLETION_TOKENS_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    total_tokens: int
    prompt_tokens: int
    completion_tokens: int
    def __init__(self, conversation_id: _Optional[str] = ..., total_tokens: _Optional[int] = ..., prompt_tokens: _Optional[int] = ..., completion_tokens: _Optional[int] = ...) -> None: ...

class AssistExecutionEvent(_message.Message):
    __slots__ = ["conversation_id", "node_count", "total_tokens", "prompt_tokens", "completion_tokens"]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    NODE_COUNT_FIELD_NUMBER: _ClassVar[int]
    TOTAL_TOKENS_FIELD_NUMBER: _ClassVar[int]
    PROMPT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COMPLETION_TOKENS_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    node_count: int
    total_tokens: int
    prompt_tokens: int
    completion_tokens: int
    def __init__(self, conversation_id: _Optional[str] = ..., node_count: _Optional[int] = ..., total_tokens: _Optional[int] = ..., prompt_tokens: _Optional[int] = ..., completion_tokens: _Optional[int] = ...) -> None: ...

class AssistNewConversationEvent(_message.Message):
    __slots__ = ["category"]
    CATEGORY_FIELD_NUMBER: _ClassVar[int]
    category: str
    def __init__(self, category: _Optional[str] = ...) -> None: ...

class AssistAccessRequest(_message.Message):
    __slots__ = ["resource_type", "total_tokens", "prompt_tokens", "completion_tokens"]
    RESOURCE_TYPE_FIELD_NUMBER: _ClassVar[int]
    TOTAL_TOKENS_FIELD_NUMBER: _ClassVar[int]
    PROMPT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COMPLETION_TOKENS_FIELD_NUMBER: _ClassVar[int]
    resource_type: str
    total_tokens: int
    prompt_tokens: int
    completion_tokens: int
    def __init__(self, resource_type: _Optional[str] = ..., total_tokens: _Optional[int] = ..., prompt_tokens: _Optional[int] = ..., completion_tokens: _Optional[int] = ...) -> None: ...

class AssistAction(_message.Message):
    __slots__ = ["action", "total_tokens", "prompt_tokens", "completion_tokens"]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    TOTAL_TOKENS_FIELD_NUMBER: _ClassVar[int]
    PROMPT_TOKENS_FIELD_NUMBER: _ClassVar[int]
    COMPLETION_TOKENS_FIELD_NUMBER: _ClassVar[int]
    action: str
    total_tokens: int
    prompt_tokens: int
    completion_tokens: int
    def __init__(self, action: _Optional[str] = ..., total_tokens: _Optional[int] = ..., prompt_tokens: _Optional[int] = ..., completion_tokens: _Optional[int] = ...) -> None: ...

class AccessListMetadata(_message.Message):
    __slots__ = ["id"]
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class AccessListCreate(_message.Message):
    __slots__ = ["metadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    metadata: AccessListMetadata
    def __init__(self, metadata: _Optional[_Union[AccessListMetadata, _Mapping]] = ...) -> None: ...

class AccessListUpdate(_message.Message):
    __slots__ = ["metadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    metadata: AccessListMetadata
    def __init__(self, metadata: _Optional[_Union[AccessListMetadata, _Mapping]] = ...) -> None: ...

class AccessListDelete(_message.Message):
    __slots__ = ["metadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    metadata: AccessListMetadata
    def __init__(self, metadata: _Optional[_Union[AccessListMetadata, _Mapping]] = ...) -> None: ...

class AccessListMemberCreate(_message.Message):
    __slots__ = ["metadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    metadata: AccessListMetadata
    def __init__(self, metadata: _Optional[_Union[AccessListMetadata, _Mapping]] = ...) -> None: ...

class AccessListMemberUpdate(_message.Message):
    __slots__ = ["metadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    metadata: AccessListMetadata
    def __init__(self, metadata: _Optional[_Union[AccessListMetadata, _Mapping]] = ...) -> None: ...

class AccessListMemberDelete(_message.Message):
    __slots__ = ["metadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    metadata: AccessListMetadata
    def __init__(self, metadata: _Optional[_Union[AccessListMetadata, _Mapping]] = ...) -> None: ...

class AccessListGrantsToUser(_message.Message):
    __slots__ = ["count_roles_granted", "count_traits_granted"]
    COUNT_ROLES_GRANTED_FIELD_NUMBER: _ClassVar[int]
    COUNT_TRAITS_GRANTED_FIELD_NUMBER: _ClassVar[int]
    count_roles_granted: int
    count_traits_granted: int
    def __init__(self, count_roles_granted: _Optional[int] = ..., count_traits_granted: _Optional[int] = ...) -> None: ...

class IntegrationEnrollMetadata(_message.Message):
    __slots__ = ["id", "kind", "user_name"]
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    id: str
    kind: IntegrationEnrollKind
    user_name: str
    def __init__(self, id: _Optional[str] = ..., kind: _Optional[_Union[IntegrationEnrollKind, str]] = ..., user_name: _Optional[str] = ...) -> None: ...

class UIIntegrationEnrollStartEvent(_message.Message):
    __slots__ = ["metadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    metadata: IntegrationEnrollMetadata
    def __init__(self, metadata: _Optional[_Union[IntegrationEnrollMetadata, _Mapping]] = ...) -> None: ...

class UIIntegrationEnrollCompleteEvent(_message.Message):
    __slots__ = ["metadata"]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    metadata: IntegrationEnrollMetadata
    def __init__(self, metadata: _Optional[_Union[IntegrationEnrollMetadata, _Mapping]] = ...) -> None: ...

class ResourceCreateEvent(_message.Message):
    __slots__ = ["resource_type", "resource_origin", "cloud_provider", "database"]
    RESOURCE_TYPE_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_ORIGIN_FIELD_NUMBER: _ClassVar[int]
    CLOUD_PROVIDER_FIELD_NUMBER: _ClassVar[int]
    DATABASE_FIELD_NUMBER: _ClassVar[int]
    resource_type: str
    resource_origin: str
    cloud_provider: str
    database: DiscoveredDatabaseMetadata
    def __init__(self, resource_type: _Optional[str] = ..., resource_origin: _Optional[str] = ..., cloud_provider: _Optional[str] = ..., database: _Optional[_Union[DiscoveredDatabaseMetadata, _Mapping]] = ...) -> None: ...

class DiscoveredDatabaseMetadata(_message.Message):
    __slots__ = ["db_type", "db_protocol"]
    DB_TYPE_FIELD_NUMBER: _ClassVar[int]
    DB_PROTOCOL_FIELD_NUMBER: _ClassVar[int]
    db_type: str
    db_protocol: str
    def __init__(self, db_type: _Optional[str] = ..., db_protocol: _Optional[str] = ...) -> None: ...

class FeatureRecommendationEvent(_message.Message):
    __slots__ = ["user_name", "feature", "feature_recommendation_status"]
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    FEATURE_FIELD_NUMBER: _ClassVar[int]
    FEATURE_RECOMMENDATION_STATUS_FIELD_NUMBER: _ClassVar[int]
    user_name: str
    feature: Feature
    feature_recommendation_status: FeatureRecommendationStatus
    def __init__(self, user_name: _Optional[str] = ..., feature: _Optional[_Union[Feature, str]] = ..., feature_recommendation_status: _Optional[_Union[FeatureRecommendationStatus, str]] = ...) -> None: ...

class UsageEventOneOf(_message.Message):
    __slots__ = ["ui_banner_click", "ui_onboard_complete_go_to_dashboard_click", "ui_onboard_add_first_resource_click", "ui_onboard_add_first_resource_later_click", "ui_onboard_set_credential_submit", "ui_onboard_register_challenge_submit", "ui_recovery_codes_continue_click", "ui_recovery_codes_copy_click", "ui_recovery_codes_print_click", "ui_discover_started_event", "ui_discover_resource_selection_event", "ui_discover_deploy_service_event", "ui_discover_database_register_event", "ui_discover_database_configure_mtls_event", "ui_discover_desktop_active_directory_tools_install_event", "ui_discover_desktop_active_directory_configure_event", "ui_discover_auto_discovered_resources_event", "ui_discover_database_configure_iam_policy_event", "ui_discover_principals_configure_event", "ui_discover_test_connection_event", "ui_discover_completed_event", "ui_create_new_role_click", "ui_create_new_role_save_click", "ui_create_new_role_cancel_click", "ui_create_new_role_view_documentation_click", "ui_discover_integration_aws_oidc_connect_event", "ui_discover_database_rds_enroll_event", "ui_call_to_action_click_event", "assist_completion", "ui_integration_enroll_start_event", "ui_integration_enroll_complete_event", "ui_onboard_questionnaire_submit", "assist_execution", "assist_new_conversation", "resource_create_event", "feature_recommendation_event", "assist_access_request", "assist_action", "access_list_create", "access_list_update", "access_list_delete", "access_list_member_create", "access_list_member_update", "access_list_member_delete", "access_list_grants_to_user", "ui_discover_ec2_instance_selection", "ui_discover_deploy_eice", "ui_discover_create_node"]
    UI_BANNER_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_ONBOARD_COMPLETE_GO_TO_DASHBOARD_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_ONBOARD_ADD_FIRST_RESOURCE_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_ONBOARD_ADD_FIRST_RESOURCE_LATER_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_ONBOARD_SET_CREDENTIAL_SUBMIT_FIELD_NUMBER: _ClassVar[int]
    UI_ONBOARD_REGISTER_CHALLENGE_SUBMIT_FIELD_NUMBER: _ClassVar[int]
    UI_RECOVERY_CODES_CONTINUE_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_RECOVERY_CODES_COPY_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_RECOVERY_CODES_PRINT_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_STARTED_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_RESOURCE_SELECTION_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_DEPLOY_SERVICE_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_DATABASE_REGISTER_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_DATABASE_CONFIGURE_MTLS_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_DESKTOP_ACTIVE_DIRECTORY_TOOLS_INSTALL_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_DESKTOP_ACTIVE_DIRECTORY_CONFIGURE_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_AUTO_DISCOVERED_RESOURCES_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_DATABASE_CONFIGURE_IAM_POLICY_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_PRINCIPALS_CONFIGURE_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_TEST_CONNECTION_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_COMPLETED_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_CREATE_NEW_ROLE_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_CREATE_NEW_ROLE_SAVE_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_CREATE_NEW_ROLE_CANCEL_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_CREATE_NEW_ROLE_VIEW_DOCUMENTATION_CLICK_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_INTEGRATION_AWS_OIDC_CONNECT_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_DATABASE_RDS_ENROLL_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_CALL_TO_ACTION_CLICK_EVENT_FIELD_NUMBER: _ClassVar[int]
    ASSIST_COMPLETION_FIELD_NUMBER: _ClassVar[int]
    UI_INTEGRATION_ENROLL_START_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_INTEGRATION_ENROLL_COMPLETE_EVENT_FIELD_NUMBER: _ClassVar[int]
    UI_ONBOARD_QUESTIONNAIRE_SUBMIT_FIELD_NUMBER: _ClassVar[int]
    ASSIST_EXECUTION_FIELD_NUMBER: _ClassVar[int]
    ASSIST_NEW_CONVERSATION_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_CREATE_EVENT_FIELD_NUMBER: _ClassVar[int]
    FEATURE_RECOMMENDATION_EVENT_FIELD_NUMBER: _ClassVar[int]
    ASSIST_ACCESS_REQUEST_FIELD_NUMBER: _ClassVar[int]
    ASSIST_ACTION_FIELD_NUMBER: _ClassVar[int]
    ACCESS_LIST_CREATE_FIELD_NUMBER: _ClassVar[int]
    ACCESS_LIST_UPDATE_FIELD_NUMBER: _ClassVar[int]
    ACCESS_LIST_DELETE_FIELD_NUMBER: _ClassVar[int]
    ACCESS_LIST_MEMBER_CREATE_FIELD_NUMBER: _ClassVar[int]
    ACCESS_LIST_MEMBER_UPDATE_FIELD_NUMBER: _ClassVar[int]
    ACCESS_LIST_MEMBER_DELETE_FIELD_NUMBER: _ClassVar[int]
    ACCESS_LIST_GRANTS_TO_USER_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_EC2_INSTANCE_SELECTION_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_DEPLOY_EICE_FIELD_NUMBER: _ClassVar[int]
    UI_DISCOVER_CREATE_NODE_FIELD_NUMBER: _ClassVar[int]
    ui_banner_click: UIBannerClickEvent
    ui_onboard_complete_go_to_dashboard_click: UIOnboardCompleteGoToDashboardClickEvent
    ui_onboard_add_first_resource_click: UIOnboardAddFirstResourceClickEvent
    ui_onboard_add_first_resource_later_click: UIOnboardAddFirstResourceLaterClickEvent
    ui_onboard_set_credential_submit: UIOnboardSetCredentialSubmitEvent
    ui_onboard_register_challenge_submit: UIOnboardRegisterChallengeSubmitEvent
    ui_recovery_codes_continue_click: UIRecoveryCodesContinueClickEvent
    ui_recovery_codes_copy_click: UIRecoveryCodesCopyClickEvent
    ui_recovery_codes_print_click: UIRecoveryCodesPrintClickEvent
    ui_discover_started_event: UIDiscoverStartedEvent
    ui_discover_resource_selection_event: UIDiscoverResourceSelectionEvent
    ui_discover_deploy_service_event: UIDiscoverDeployServiceEvent
    ui_discover_database_register_event: UIDiscoverDatabaseRegisterEvent
    ui_discover_database_configure_mtls_event: UIDiscoverDatabaseConfigureMTLSEvent
    ui_discover_desktop_active_directory_tools_install_event: UIDiscoverDesktopActiveDirectoryToolsInstallEvent
    ui_discover_desktop_active_directory_configure_event: UIDiscoverDesktopActiveDirectoryConfigureEvent
    ui_discover_auto_discovered_resources_event: UIDiscoverAutoDiscoveredResourcesEvent
    ui_discover_database_configure_iam_policy_event: UIDiscoverDatabaseConfigureIAMPolicyEvent
    ui_discover_principals_configure_event: UIDiscoverPrincipalsConfigureEvent
    ui_discover_test_connection_event: UIDiscoverTestConnectionEvent
    ui_discover_completed_event: UIDiscoverCompletedEvent
    ui_create_new_role_click: UICreateNewRoleClickEvent
    ui_create_new_role_save_click: UICreateNewRoleSaveClickEvent
    ui_create_new_role_cancel_click: UICreateNewRoleCancelClickEvent
    ui_create_new_role_view_documentation_click: UICreateNewRoleViewDocumentationClickEvent
    ui_discover_integration_aws_oidc_connect_event: UIDiscoverIntegrationAWSOIDCConnectEvent
    ui_discover_database_rds_enroll_event: UIDiscoverDatabaseRDSEnrollEvent
    ui_call_to_action_click_event: UICallToActionClickEvent
    assist_completion: AssistCompletionEvent
    ui_integration_enroll_start_event: UIIntegrationEnrollStartEvent
    ui_integration_enroll_complete_event: UIIntegrationEnrollCompleteEvent
    ui_onboard_questionnaire_submit: UIOnboardQuestionnaireSubmitEvent
    assist_execution: AssistExecutionEvent
    assist_new_conversation: AssistNewConversationEvent
    resource_create_event: ResourceCreateEvent
    feature_recommendation_event: FeatureRecommendationEvent
    assist_access_request: AssistAccessRequest
    assist_action: AssistAction
    access_list_create: AccessListCreate
    access_list_update: AccessListUpdate
    access_list_delete: AccessListDelete
    access_list_member_create: AccessListMemberCreate
    access_list_member_update: AccessListMemberUpdate
    access_list_member_delete: AccessListMemberDelete
    access_list_grants_to_user: AccessListGrantsToUser
    ui_discover_ec2_instance_selection: UIDiscoverEC2InstanceSelectionEvent
    ui_discover_deploy_eice: UIDiscoverDeployEICEEvent
    ui_discover_create_node: UIDiscoverCreateNodeEvent
    def __init__(self, ui_banner_click: _Optional[_Union[UIBannerClickEvent, _Mapping]] = ..., ui_onboard_complete_go_to_dashboard_click: _Optional[_Union[UIOnboardCompleteGoToDashboardClickEvent, _Mapping]] = ..., ui_onboard_add_first_resource_click: _Optional[_Union[UIOnboardAddFirstResourceClickEvent, _Mapping]] = ..., ui_onboard_add_first_resource_later_click: _Optional[_Union[UIOnboardAddFirstResourceLaterClickEvent, _Mapping]] = ..., ui_onboard_set_credential_submit: _Optional[_Union[UIOnboardSetCredentialSubmitEvent, _Mapping]] = ..., ui_onboard_register_challenge_submit: _Optional[_Union[UIOnboardRegisterChallengeSubmitEvent, _Mapping]] = ..., ui_recovery_codes_continue_click: _Optional[_Union[UIRecoveryCodesContinueClickEvent, _Mapping]] = ..., ui_recovery_codes_copy_click: _Optional[_Union[UIRecoveryCodesCopyClickEvent, _Mapping]] = ..., ui_recovery_codes_print_click: _Optional[_Union[UIRecoveryCodesPrintClickEvent, _Mapping]] = ..., ui_discover_started_event: _Optional[_Union[UIDiscoverStartedEvent, _Mapping]] = ..., ui_discover_resource_selection_event: _Optional[_Union[UIDiscoverResourceSelectionEvent, _Mapping]] = ..., ui_discover_deploy_service_event: _Optional[_Union[UIDiscoverDeployServiceEvent, _Mapping]] = ..., ui_discover_database_register_event: _Optional[_Union[UIDiscoverDatabaseRegisterEvent, _Mapping]] = ..., ui_discover_database_configure_mtls_event: _Optional[_Union[UIDiscoverDatabaseConfigureMTLSEvent, _Mapping]] = ..., ui_discover_desktop_active_directory_tools_install_event: _Optional[_Union[UIDiscoverDesktopActiveDirectoryToolsInstallEvent, _Mapping]] = ..., ui_discover_desktop_active_directory_configure_event: _Optional[_Union[UIDiscoverDesktopActiveDirectoryConfigureEvent, _Mapping]] = ..., ui_discover_auto_discovered_resources_event: _Optional[_Union[UIDiscoverAutoDiscoveredResourcesEvent, _Mapping]] = ..., ui_discover_database_configure_iam_policy_event: _Optional[_Union[UIDiscoverDatabaseConfigureIAMPolicyEvent, _Mapping]] = ..., ui_discover_principals_configure_event: _Optional[_Union[UIDiscoverPrincipalsConfigureEvent, _Mapping]] = ..., ui_discover_test_connection_event: _Optional[_Union[UIDiscoverTestConnectionEvent, _Mapping]] = ..., ui_discover_completed_event: _Optional[_Union[UIDiscoverCompletedEvent, _Mapping]] = ..., ui_create_new_role_click: _Optional[_Union[UICreateNewRoleClickEvent, _Mapping]] = ..., ui_create_new_role_save_click: _Optional[_Union[UICreateNewRoleSaveClickEvent, _Mapping]] = ..., ui_create_new_role_cancel_click: _Optional[_Union[UICreateNewRoleCancelClickEvent, _Mapping]] = ..., ui_create_new_role_view_documentation_click: _Optional[_Union[UICreateNewRoleViewDocumentationClickEvent, _Mapping]] = ..., ui_discover_integration_aws_oidc_connect_event: _Optional[_Union[UIDiscoverIntegrationAWSOIDCConnectEvent, _Mapping]] = ..., ui_discover_database_rds_enroll_event: _Optional[_Union[UIDiscoverDatabaseRDSEnrollEvent, _Mapping]] = ..., ui_call_to_action_click_event: _Optional[_Union[UICallToActionClickEvent, _Mapping]] = ..., assist_completion: _Optional[_Union[AssistCompletionEvent, _Mapping]] = ..., ui_integration_enroll_start_event: _Optional[_Union[UIIntegrationEnrollStartEvent, _Mapping]] = ..., ui_integration_enroll_complete_event: _Optional[_Union[UIIntegrationEnrollCompleteEvent, _Mapping]] = ..., ui_onboard_questionnaire_submit: _Optional[_Union[UIOnboardQuestionnaireSubmitEvent, _Mapping]] = ..., assist_execution: _Optional[_Union[AssistExecutionEvent, _Mapping]] = ..., assist_new_conversation: _Optional[_Union[AssistNewConversationEvent, _Mapping]] = ..., resource_create_event: _Optional[_Union[ResourceCreateEvent, _Mapping]] = ..., feature_recommendation_event: _Optional[_Union[FeatureRecommendationEvent, _Mapping]] = ..., assist_access_request: _Optional[_Union[AssistAccessRequest, _Mapping]] = ..., assist_action: _Optional[_Union[AssistAction, _Mapping]] = ..., access_list_create: _Optional[_Union[AccessListCreate, _Mapping]] = ..., access_list_update: _Optional[_Union[AccessListUpdate, _Mapping]] = ..., access_list_delete: _Optional[_Union[AccessListDelete, _Mapping]] = ..., access_list_member_create: _Optional[_Union[AccessListMemberCreate, _Mapping]] = ..., access_list_member_update: _Optional[_Union[AccessListMemberUpdate, _Mapping]] = ..., access_list_member_delete: _Optional[_Union[AccessListMemberDelete, _Mapping]] = ..., access_list_grants_to_user: _Optional[_Union[AccessListGrantsToUser, _Mapping]] = ..., ui_discover_ec2_instance_selection: _Optional[_Union[UIDiscoverEC2InstanceSelectionEvent, _Mapping]] = ..., ui_discover_deploy_eice: _Optional[_Union[UIDiscoverDeployEICEEvent, _Mapping]] = ..., ui_discover_create_node: _Optional[_Union[UIDiscoverCreateNodeEvent, _Mapping]] = ...) -> None: ...
