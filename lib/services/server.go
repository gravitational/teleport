package services

import (
	"time"
)

// Server represents a Node, Proxy or Auth server in a Teleport cluster
type Server interface {
	// GetName returns server name
	GetName() string
	// GetAddr return server address
	GetAddr() string
	// GetHostname returns server hostname
	GetHostname() string
	// GetNamespace returns server namespace
	GetNamespace() string
	// GetAllLabels returns server's static and dynamic label values merged together
	GetAllLabels() map[string]string
	// GetLabels returns server's static label key pairs
	GetLabels() map[string]string
	// GetCmdLabels returns command labels
	GetCmdLabels() map[string]CommandLabel
	// String returns string representation of the server
	String() string
}

// ServerV1 is version1 resource spec of the server
type ServerV1 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is version
	Version string `json:"version"`
	// Metadata is User metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains user specification
	Spec ServerSpecV1 `json:"spec"`
}

// GetName returns server name
func (s *ServerV1) GetName() string {
	return s.Metadata.Name
}

// GetAddr return server address
func (s *ServerV1) GetAddr() string {
	return s.Spec.Addr
}

// GetHostname returns server hostname
func (s *ServerV1) GetHostname() string {
	return s.Spec.Hostname
}

// GetLabels returns server's static label key pairs
func (s *ServerV1) GetLabels() map[string]string {
	return s.Spec.Labels
}

// GetCmdLabels returns command labels
func (s *ServerV1) GetCmdLabels() map[string]CommandLabel {
	if s.Spec.CmdLabels == nil {
		return nil
	}
	out := make(map[string]CommandLabel, len(s.s.Spec.CmdLabels))
	for key := range s.Spec.CmdLabels {
		val := s.Spec.CmdLabels[key]
		out[key] = &val
	}
	return out
}

func (s *ServerV1) String() string {
	return fmt.Sprintf("Server(name=%v, namespace=%v, addr=%v, labels=%v)", s.Metadata.Name, s.Namespace, s.Addr, s.Labels)
}

// GetNamespace returns server namespace
func (s *ServerV1) GetNamespace() string {
	return ProcessNamespace(s.Metadata.Namespace)
}

// GetAllLabels returns the full key:value map of both static labels and
// "command labels"
func (s *ServerV1) GetAllLabels() map[string]string {
	lmap := make(map[string]string)
	for key, value := range s.Labels {
		lmap[key] = value
	}
	for key, cmd := range s.CmdLabels {
		lmap[key] = cmd.Result
	}
	return lmap
}

// MatchAgainst takes a map of labels and returns True if this server
// has ALL of them
//
// Any server matches against an empty label set
func (s *ServerV1) MatchAgainst(labels map[string]string) bool {
	if labels != nil {
		myLabels := s.LabelsMap()
		for key, value := range labels {
			if myLabels[key] != value {
				return false
			}
		}
	}
	return true
}

// LabelsString returns a comma separated string with all node's labels
func (s *ServerV1) LabelsString() string {
	labels := []string{}
	for key, val := range s.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", key, val))
	}
	for key, val := range s.CmdLabels {
		labels = append(labels, fmt.Sprintf("%s=%s", key, val.Result))
	}
	sort.Strings(labels)
	return strings.Join(labels, ",")
}

// ServerSpecV1 is a specification for V1 Server
type ServerSpecV1 struct {
	// Addr is server host:port address
	Addr string `json:"addr"`
	// Hostname is server hostname
	Hostname string `json:"hostname"`
	// Labels is server static labels
	Labels map[string]string `json:"labels,omitempty"`
	// CmdLabels is server dynamic labels
	CmdLabels map[string]CommandLabelV1 `json:"cmd_labels,omitempty"`
}

// ServerSpecV1SchemaTemplate is JSON schema for server
const ServerSpecV1SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "addr": {"type": "string"},
    "hostname": {"type": "string"},
    "labels": {
      "type": "object"
      "patternProperties": {
        "^.*$":  { "type": "string" }
      }
    },
    "cmd_labels": {
      "type": "object"
       "patternProperties": {
         "^.*$": { 
           "type": "object",
           "additionalProperties": false,
           "required": ["command"],
           "properties": {
             "command": {"type": "array", "items": {"type": "string"}},
             "period": {"type": "string"},
             "result": {"type": "string"}
           }
         }
       }
    }
  }
}`

// ServerV0 represents V0 spec of the server
type ServerV0 struct {
	Kind      string                    `json:"kind"`
	ID        string                    `json:"id"`
	Addr      string                    `json:"addr"`
	Hostname  string                    `json:"hostname"`
	Namespace string                    `json:"namespace"`
	Labels    map[string]string         `json:"labels"`
	CmdLabels map[string]CommandLabelV0 `json:"cmd_labels"`
}

// V1 returns V1 version
func (s *ServerV0) V1() *ServerV1 {
	labels := make(map[string]CommandLabelV1, len(s.CmdLabels))
	for key := range s.CmdLabels {
		val := s.CmdLabels[key]
		labels[key] = CommandLabelV1{
			Period:  Duration{Duration: val.Period},
			Result:  val.Result,
			Command: val.Command,
		}
	}
	return &ServerV1{
		Kind:    s.Kind,
		Version: V1,
		Metadata: Metadata{
			Name:      s.ID,
			Namespace: s.Namespace,
		},
		Spec: ServerSpecV1{
			Addr:     s.Addr,
			Hostname: s.Hostname,
			Labels:   labels,
		},
	}
}

// CommandLabelV1 is a label that has a value as a result of the
// output generated by running command, e.g. hostname
type CommandLabel interface {
	// GetPeriod returns label period
	GetPeriod() time.Duration
	// GetResult returns label result
	GetResult() string
	// GetCommand returns to execute and set as a label result
	GetCommand() []string
}

// CommandLabelV1 is a label that has a value as a result of the
// output generated by running command, e.g. hostname
type CommandLabelV1 struct {
	// Period is a time between command runs
	Period Duration `json:"period"`
	// Command is a command to run
	Command []string `json:"command"` //["/usr/bin/hostname", "--long"]
	// Result captures standard output
	Result string `json:"result"`
}

// GetPeriod returns label period
func (c *CommandLabelV1) GetPeriod() time.Duration {
	return c.Period.Duration
}

// GetResult returns label result
func (c *CommandLabelV1) GetResult() string {
	return c.Result
}

// GetCommand returns to execute and set as a label result
func (c *CommandLabelV1) GetCommand() []string {
	return c.Command
}

// CommandLabelV0 is a label that has a value as a result of the
// output generated by running command, e.g. hostname
type CommandLabelV0 struct {
	// Period is a time between command runs
	Period time.Duration `json:"period"`
	// Command is a command to run
	Command []string `json:"command"` //["/usr/bin/hostname", "--long"]
	// Result captures standard output
	Result string `json:"result"`
}

// CommandLabels is a set of command labels
type CommandLabels map[string]CommandLabel

// SetEnv sets the value of the label from environment variable
func (c *CommandLabels) SetEnv(v string) error {
	if err := json.Unmarshal([]byte(v), c); err != nil {
		return trace.Wrap(err, "can not parse Command Labels")
	}
	return nil
}
