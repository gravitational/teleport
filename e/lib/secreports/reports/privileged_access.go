package reports

// PrivilegeAccessReport provides a report for privileged access.
var PrivilegeAccessReport = &AuditReportType{
	Version: "0.0.1",
	Name:    "privilege_access_report",
	Title:   "Privileged Access Report",
	Queries: []AuditQueryType{
		{
			Name:        "database_sessions_with_weak_security",
			Title:       "Database sessions with weak security",
			Description: "Database session count per user that doesn't use Access Request or MFA access",
			Query: `
SELECT
	event_date,
	count(*) as count,
	user
FROM
	db_session_start
WHERE 
    CARDINALITY(access_requests) IS NULL
AND 
    with_mfa IS NULL
AND 
    impersonator IS NULL
AND 
    trusted_device_device_id IS NULL
GROUP BY 
    event_date,
    user
ORDER BY
    event_date 
`,
		},
		{
			Name:        "kube_execs",
			Title:       "Kube Execs",
			Description: "Eliminate usage of kube exec",
			Query: `
SELECT
	event_date,
	count(*) as count,
	user
FROM 
    session_start 
WHERE
    CARDINALITY(access_requests) IS NULL
AND 
    with_mfa IS NULL
AND 
    impersonator IS NULL
AND 
    trusted_device_device_id IS NULL
AND
    proto='kube'
GROUP BY
    event_date,
    proto,
    user
ORDER BY
    event_date
`,
		},
		{
			Name:        "kube_access_with_weak_security",
			Title:       "Kubernetes API calls with weak security",
			Description: "Setup access requests, device trust and per-session MFA",
			Query: `
SELECT 
    event_date,
    count(*) as count,
    user
FROM 
    kube_request 
WHERE 
    CARDINALITY(access_requests) IS NULL
AND 
    with_mfa IS NULL
AND 
    impersonator IS NULL
AND 
    trusted_device_device_id IS NULL
GROUP BY 
    event_date,
    user
ORDER BY
    event_date 
`,
		},
		{
			Name:        "ssh_sessions_with_weak_security",
			Title:       "SSH sessions with weak security",
			Description: "Setup access requests, device trust and per-session MFA",
			Query: `
SELECT
	event_date,
	count(*) as count,
	user
FROM 
    session_start 
WHERE
    CARDINALITY(access_requests) IS NULL
AND
    proto='ssh'
AND 
    with_mfa IS NULL
AND 
    impersonator IS NULL
AND 
    trusted_device_device_id IS NULL
GROUP BY
    event_date,
    proto,
    user
ORDER BY
    event_date
`,
		},
		{
			Name:        "cert_expiration_more_than_1d",
			Title:       "Long lived certificates",
			Description: "Use short lived certificates less than a working day. Check out Machine ID to get short lived certificates for your automation",
			Query: `
SELECT DISTINCT 
    event_date,
    COUNT(*) AS count,
	identity_user AS user
FROM 
    cert_create
WHERE 
    from_iso8601_timestamp(identity_expires) - from_iso8601_timestamp(time) > INTERVAL '1' DAY
GROUP BY 
    event_date, 
    identity_user
ORDER BY 
    event_date
`,
		},
		{
			Name:        "db_postgres_user",
			Title:       "Privileged Postgres sessions",
			Description: "Setup access requests, device trust and per-session MFA",
			Query: `
SELECT
	event_date,
	COUNT(*) AS count,
	user
FROM 
    db_session_start 
GROUP BY
    event_date,
    user
ORDER BY
    event_date
`,
		},
		{
			Name:        "instance_join_token_less_than_1d",
			Title:       "Long-lived join tokens",
			Description: "Use short-lived tokens to reduce risk of compromise or AWS/GCP/Azure joining when possible",
			Query: `
SELECT
	event_date,
	COUNT(*) as count,
	node_name,
	host_id
FROM instance_join 
WHERE 
	FROM_ISO8601_TIMESTAMP(token_expires) - FROM_ISO8601_TIMESTAMP(time) > INTERVAL '1' DAY
GROUP BY
    event_date,
    node_name,
    host_id
ORDER BY
    event_date
`,
		},
		{
			Name:        "session_start_root_user",
			Title:       "Root SSH sessions",
			Description: "Don’t use `root` for SSH sessions, downgrade access to users with sudo privileges instead",
			Query: `
SELECT
	event_date,
	COUNT(*) as count,
	user
FROM
	session_start 
WHERE 
    login='root'
GROUP BY 
    event_date, 
    user
ORDER BY
    event_date
 `,
		},
		{
			Name:        "kube_system_api_calls",
			Title:       "System Kubernetes API calls",
			Description: "Don't use system:masters group for Kubernetes API calls, downgrade to `view` or `edit` instead",
			Query: `
SELECT
	event_date,
	COUNT(*) as count,
	user
FROM
	kube_request 
WHERE 
    CONTAINS(kubernetes_groups, 'system:masters')
GROUP BY 
    event_date, 
    user
ORDER BY
    event_date
 `,
		},
	},
}
