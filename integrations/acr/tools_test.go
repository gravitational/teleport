package acr

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// testAuditLogs is a sample payload matching exactly what the discoveryLog
// handler produces from SSMRun events via the ssmRunEntry struct.
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
    "invocation_url": "https://us-west-2.console.aws.amazon.com/systems-manager/run-command/db214026-b79c-4174-a564-ca2ca714830e/i-fake068956d2ebf2f",
    "stdout": "Downloading teleport...",
    "stderr": "size of download (217917539 bytes) exceeds available disk space (44501376 bytes)"
  },
  {
    "account_id": "fake49123960",
    "region": "us-west-2",
    "instance_id": "i-fake068956d2ebf2f",
    "status": "Failed",
    "exit_code": 1,
    "command_id": "3d5c7faa-a598-4fe1-b00a-2db20ab172a6",
    "invocation_url": "https://us-west-2.console.aws.amazon.com/systems-manager/run-command/3d5c7faa-a598-4fe1-b00a-2db20ab172a6/i-fake068956d2ebf2f",
    "stdout": "Offloading the installation part to the generic Teleport install script hosted at: https://discover-dev-5.cloud.gravitational.io:443/scripts/install.sh\nDownloading from https://cdn.teleport.dev/teleport-ent-v18.6.5-linux-amd64-bin.tar.gz and extracting teleport to /tmp.LveeHtOGnU ...\nThe install script (/tmp/tmp.fzF5bRzL5G) returned a non-zero exit code\n",
    "stderr": "tar: teleport-ent/teleport-update: Wrote only 7680 of 10240 bytes\ntar: Exiting with failure status due to previous errors\nfailed to run commands: exit status 1"
  }
]`

// TestClassify calls the real OpenAI API with sample audit logs.
// Skipped unless OPENAI_API_KEY is set.
//
//	OPENAI_API_KEY=sk-... go test ./integrations/acr/ -run TestClassify -v
func TestClassify(t *testing.T) {
	// @TODO: Remove this after live testing is done
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
