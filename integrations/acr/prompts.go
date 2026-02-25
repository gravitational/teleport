package acr

// classifySystemPrompt instructs the LLM on how to classify audit log events.
const classifySystemPrompt = `You are a Teleport infrastructure analyst. Classify audit log events by grouping them hierarchically.

Grouping rules:
1. First group by account_id and region.
2. Within each account/region, group by instance_id.
3. Within each instance, group by error type. Deduplicate: if the same error occurs multiple times on one instance, merge into a single issue with the total event count.

For each issue, assign:
- confidence: high (root cause is unambiguous), medium (likely correct but inferred), low (best guess, logs are vague).
- count: number of events for this issue on this instance.
- error_summary: a short plain-language description of what went wrong.
- remediation: concrete steps to fix it.

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

