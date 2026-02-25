package acr

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// testAuditLogs is a sample payload matching what the discoveryLog handler
// produces from SSMRun events.
const testAuditLogs = `[
  {
    "account_id": "fake58854585",
    "region": "us-east-1",
    "instance_id": "i-fake3a3ef14cddf5b",
    "status": "EC2 Instance is not registered in SSM. Make sure that the instance has AmazonSSMManagedInstanceCore policy assigned.",
    "exit_code": -1,
    "command_id": "no-command",
    "invocation_url": "",
    "stdout": "",
    "stderr": ""
  },
  {
    "account_id": "fake49123960",
    "region": "us-west-2",
    "instance_id": "i-fake068956d2ebf2f",
    "status": "Failed",
    "exit_code": 1,
    "command_id": "db214026-b79c-4174-a564-ca2ca714830e",
    "invocation_url": "https://us-west-2.console.aws.amazon.com/systems-manager/run-command/db214026-b79c-4174-a564-ca2ca714830e/i-0b5e068956d2ebf2f",
    "stdout": "Downloading teleport...",
    "stderr": "size of download exceeds available disk space"
  },
	{
  "account_id": "fake49123960",
  "cluster_name": "discover-dev-5.cloud.gravitational.io",
  "code": "TDS00W",
  "command_id": "3d5c7faa-a598-4fe1-b00a-2db20ab172a6",
  "ei": 0,
  "event": "ssm.run",
  "exit_code": 1,
  "instance_id": "i-fake068956d2ebf2f",
  "invocation_url": "https://us-west-2.console.aws.amazon.com/systems-manager/run-command/3d5c7faa-a598-4fe1-b00a-2db20ab172a6/i-0b5e068956d2ebf2f",
  "region": "us-west-2",
  "status": "Failed",
  "stderr": "  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current\n                                 Dload  Upload   Total   Spent    Left  Speed\n\r  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0\r  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0\r 13  207M   13 28.9M    0     0  18.0M      0  0:00:11  0:00:01  0:00:10 18.0M\r 25  207M   25 52.0M    0     0  20.0M      0  0:00:10  0:00:02  0:00:08 20.0M\r 32  207M   32 66.7M    0     0  20.0M      0  0:00:10  0:00:03  0:00:07 20.0M\r 44  207M   44 93.1M    0     0  20.9M      0  0:00:09  0:00:04  0:00:05 20.9M\r 52  207M   52  109M    0     0  19.8M      0  0:00:10  0:00:05  0:00:05 21.2M\r 62  207M   62  129M    0     0  20.1M      0  0:00:10  0:00:06  0:00:04 20.8M\r 68  207M   68  142M    0     0  19.7M      0  0:00:10  0:00:07  0:00:03 19.6M\r 83  207M   83  172M    0     0  20.4M      0  0:00:10  0:00:08  0:00:02 20.7M\r 92  207M   92  192M    0     0  20.8M      0  0:00:09  0:00:09 --:--:-- 20.6Mtar: teleport-ent/teleport-update: Wrote only 7680 of 10240 bytes\n\r100  207M  100  207M    0     0  20.2M      0  0:00:10  0:00:10 --:--:-- 20.7M\r100  207M  100  207M    0     0  20.2M      0  0:00:10  0:00:10 --:--:-- 20.4M\ntar: Exiting with failure status due to previous errors\nfailed to run commands: exit status 1",
  "stdout": "Offloading the installation part to the generic Teleport install script hosted at: https://discover-dev-5.cloud.gravitational.io:443/scripts/install.sh\nDownloading from https://cdn.teleport.dev/teleport-ent-v18.6.5-linux-amd64-bin.tar.gz and extracting teleport to /tmp.LveeHtOGnU ...\nThe install script (/tmp/tmp.fzF5bRzL5G) returned a non-zero exit code\n",
  "time": "2026-02-25T21:05:24.288Z",
  "uid": "1fc4bf7b-5a52-4c3e-942a-66876dc0044a"
}
]`

// TestClassify calls the real OpenAI API with sample audit logs.
// Skipped unless OPENAI_API_KEY is set.
//
//	OPENAI_API_KEY=sk-... go test ./integrations/acr/ -run TestClassify -v
func TestClassify(t *testing.T) {
	if os.Getenv(openAIKeyEnv) == "" {
		t.Skipf("%s not set, skipping integration test", openAIKeyEnv)
	}

	svc, err := NewService()
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := svc.Classify(ctx, testAuditLogs)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	// Pretty-print for eyeballing the output.
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	t.Logf("Classify result:\n%s", out)

	// Basic structural assertions.
	if result.TotalEvents == 0 {
		t.Error("expected TotalEvents > 0")
	}
	if len(result.Accounts) == 0 {
		t.Fatal("expected at least one account group")
	}

	acct := result.Accounts[0]
	if acct.AccountID == "" {
		t.Error("expected non-empty AccountID")
	}
	if acct.Region == "" {
		t.Error("expected non-empty Region")
	}
	if len(acct.Instances) == 0 {
		t.Fatal("expected at least one instance")
	}

	for _, inst := range acct.Instances {
		if inst.InstanceID == "" {
			t.Error("expected non-empty InstanceID")
		}
		if len(inst.Issues) == 0 {
			t.Errorf("instance %s: expected at least one issue", inst.InstanceID)
		}
		for _, issue := range inst.Issues {
			if issue.ErrorSummary == "" {
				t.Errorf("instance %s: issue has empty error_summary", inst.InstanceID)
			}
			if issue.Remediation == "" {
				t.Errorf("instance %s: issue has empty remediation", inst.InstanceID)
			}
		}
	}
}
