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

package auth

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/authentication/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kubernetestoken"
)

type kubernetesTokenValidator interface {
	Validate(context.Context, string) (*v1.UserInfo, error)
}

func (a *Server) checkKubernetesJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) error {
	if req.IDToken == "" {
		return trace.BadParameter("IDToken not provided for Kubernetes join request")
	}
	pt, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return trace.Wrap(err)
	}

	userInfo, err := a.kubernetesTokenValidator.Validate(ctx, req.IDToken)
	if err != nil {
		return trace.WrapWithMessage(err, "failed to validate the Kubernetes token")
	}

	log.WithFields(logrus.Fields{
		"userInfo": userInfo,
		"token":    pt.GetName(),
	}).Info("Kubernetes workload trying to join cluster")

	return trace.Wrap(checkKubernetesAllowRules(pt, userInfo))
}

func checkKubernetesAllowRules(pt types.ProvisionToken, userInfo *v1.UserInfo) error {
	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("kubernetes join method only supports ProvisionTokenV2, '%T' was provided", pt)
	}

	// If a single rule passes, accept the token
	for _, rule := range token.Spec.Kubernetes.Allow {
		if fmt.Sprintf("%s:%s", kubernetestoken.ServiceAccountNamePrefix, rule.ServiceAccount) != userInfo.Username {
			continue
		}
		return nil
	}

	return trace.AccessDenied("kubernetes token user info did not match any allow rules")
}
