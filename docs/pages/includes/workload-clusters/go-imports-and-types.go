package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadcluster "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/api/types"
)

// TbotConfig defines a configuration for running tbot.
type TbotConfig struct {
	// Version is the configuration version.
	Version string `json:"version"`
	// Oneshot determines if tbot runs as a service.
	Oneshot bool `json:"oneshot"`
	// ProxyServer is the Teleport Proxy to run tbot against.
	ProxyServer string `json:"proxy_server"`
	// Onboarding defines how tbot should attempt to join the Teleport cluster.
	Onboarding Onboarding `json:"onboarding"`
	// Storage instructs tbot where to save its internal certificates.
	Storage Storage `json:"storage"`
	// Services defines which services for tbot to run.
	Services []Service `json:"services"`
}

// Onboarding defines how tbot should attempt to join the Teleport cluster.
type Onboarding struct {
	// JoinMethod is how to join, such as iam.
	JoinMethod string `json:"join_method"`
	// Token is which token in the Teleport cluster to use.
	Token string `json:"token"`
}

// Storage instructs tbot where to save its internal certificates.
type Storage struct {
	// Type is the storage type, such as "memory" for in-memory storage.
	Type string `json:"type"`
}

// Service defines which services for tbot to run.
type Service struct {
	// Type is the service type, such as "identity".
	Type string `json:"type"`
	// Destination is used by the identity service to save retrieved identity files and certs.
	Destination Destination `json:"destination"`
}

// Destination is used by the identity service to save retrieved identity files and certs.
type Destination struct {
	// Type is the type of storage, such as "path".
	Type string `json:"type"`
	// Path is the filepath to use.
	Path string `json:"path"`
}
