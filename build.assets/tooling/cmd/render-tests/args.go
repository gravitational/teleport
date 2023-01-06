/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

type reportMode int

const (
	byPackage reportMode = 0
	byTest    reportMode = iota
)

const (
	byPackageName = "package"
	byTestName    = "test"
)

func (m *reportMode) String() string {
	switch *m {
	case byPackage:
		return byPackageName

	case byTest:
		return byTestName

	default:
		return fmt.Sprintf("Unknown filter mode %d", *m)
	}
}

func (m *reportMode) Set(text string) error {
	switch strings.TrimSpace(text) {
	case byPackageName:
		(*m) = byPackage

	case byTestName:
		(*m) = byTest

	default:
		return trace.Errorf("Invalid report mode %q", text)
	}

	return nil
}

type args struct {
	report                      reportMode
	prometheusURL               string
	prometheusTypeLabelValue    string
	prometheusBranchLabelValue  string
	prometheusJobName           string
	prometheusUser              string
	prometheusPassword          string
	prometheusReportIndFailures bool
}

func parseCommandLine() (args, error) {
	a := args{report: byPackage}
	flag.Var(&a.report, "report-by",
		fmt.Sprintf("test reporting mode [%s, %s]", byPackageName, byTestName))

	flag.StringVar(&a.prometheusURL, "prmt-url", "", "Prometheus Push Gateway URL")
	flag.StringVar(&a.prometheusJobName, "prmt-job-name", "test", "Prometheus job name")
	flag.StringVar(&a.prometheusTypeLabelValue, "prmt-type", "", "Prometheus type label value (unit, integration, etc)")
	flag.StringVar(&a.prometheusBranchLabelValue, "prmt-branch", "", "Prometheus branch label value (master, etc)")
	flag.StringVar(&a.prometheusUser, "prmt-user", "", "Prometheus Basic Auth user")
	flag.StringVar(&a.prometheusPassword, "prmt-password", "", "Prometheus Basic Auth password")
	flag.BoolVar(&a.prometheusReportIndFailures, "prmt-report-individual-failures", false, "Report individually failed tests to Prometheus")

	flag.Parse()

	if a.prometheusURL != "" {
		if a.prometheusTypeLabelValue == "" {
			return a, trace.Errorf("--prmt-type is required when --prmt-url is set")
		}

		if a.prometheusBranchLabelValue == "" {
			return a, trace.Errorf("--prmt-branch is required when --prmt-url is set")
		}
	}

	return a, nil
}
