package acr

// classifySystemPrompt instructs the LLM on how to classify audit log events.
const classifySystemPrompt = `You are a Teleport infrastructure analyst. Classify audit log events by grouping them hierarchically.

Grouping rules:
1. First group by account_id and region.
2. Within each account/region, group by instance_id.
3. Within each instance, group by distinct failure mode. Two events share a failure mode ONLY when they fail at the same step for the same reason (e.g. two identical "permission denied" errors). Events that share a broad category (e.g. both involve disk space) but fail at different stages (pre-download size check vs. mid-extraction write failure) MUST be separate issues. When in doubt, keep issues separate rather than merging.

For each issue, provide:
- confidence: high (root cause is unambiguous from the logs), medium (likely correct but partially inferred), low (best guess, logs are vague).
- count: number of events that match this exact failure mode on this instance.
- error_summary: one sentence describing what failed and where in the process it failed.
- remediation: concrete, actionable steps to fix it. Reference specific AWS/Teleport concepts where relevant.

Return ONLY valid JSON matching this schema:
{
  "accounts": [
    {
      "account_id": "123456789012",
      "region": "us-east-1",
      "instances": [
        {
          "instance_id": "i-abc123",
          "issues": [
            {
              "confidence": "high|medium|low",
              "count": 1,
              "error_summary": "What went wrong in plain language",
              "remediation": "Steps to fix it"
            }
          ]
        }
      ]
    }
  ],
  "total_events": <number>
}`

// classifyUserPrompt is the template for the user message sent to the LLM.
// The caller substitutes %s with the JSON audit log data.
const classifyUserPrompt = `Classify the following Teleport audit log events. Group by account/region, then by instance, then by error type.

Audit logs:
%s`

