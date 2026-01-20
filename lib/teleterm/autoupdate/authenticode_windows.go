package autoupdate

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/gravitational/trace"
)

func verifySignature(updatePath string) error {
	servicePath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	serviceSignature, err := readSignature(servicePath)
	if err != nil {
		return trace.Wrap(err)
	}
	if serviceSignature.Status != "Valid" || serviceSignature.Subject == "" {
		log.Info("Service binary not signed; skipping installer signature verification")
		return nil
	}

	updateSignature, err := readSignature(updatePath)
	if err != nil {
		return trace.Wrap(err)
	}
	if updateSignature.Status != "Valid" {
		return trace.BadParameter("installer signature is not valid")
	}
	if updateSignature.Subject == "" {
		return trace.BadParameter("installer signature subject is empty")
	}
	if updateSignature.Subject != serviceSignature.Subject {
		return trace.BadParameter("installer signature subject does not match service signature")
	}
	return nil
}

type signature struct {
	Status  string `json:"Status"`  // Valid, NotSigned, etc.
	Subject string `json:"Subject"` // "CN=My Company, O=..."
}

func readSignature(path string) (*signature, error) {
	psScript := `
    $path = $args[0]
    $sig = Get-AuthenticodeSignature -LiteralPath $path

    $obj = @{
       Status = $sig.Status.ToString()
       Subject = if ($sig.SignerCertificate) { $sig.SignerCertificate.Subject } else { "" }
    }
    $obj | ConvertTo-Json -Compress
`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript, path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, trace.Wrap(err, "powershell error, output: %s", output)
	}

	var info signature
	if err = json.Unmarshal(output, &info); err != nil {
		return nil, trace.Wrap(err, "failed to parse json: %s", output)
	}

	return &info, nil
}
