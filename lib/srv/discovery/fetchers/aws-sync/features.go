/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package aws_sync

const (
	awsNamespace = "aws"
	awsUsers     = awsNamespace + "/" + "users"
	awsGroups    = awsNamespace + "/" + "groups"
	awsEC2       = awsNamespace + "/" + "ec2"
	awsEKS       = awsNamespace + "/" + "eks"
	awsRDS       = awsNamespace + "/" + "rds"
	awsS3        = awsNamespace + "/" + "s3"
	awsRoles     = awsNamespace + "/" + "roles"
)

// Features is the list of supported resources by the server.
type Features struct {
	// Groups enables AWS groups sync.
	Groups bool
	// Roles enables AWS roles sync.
	Roles bool
	// Users enables AWS Users sync.
	Users bool
	// EC2 enables AWS EC2 sync.
	EC2 bool
	// RDS enables AWS RDS sync.
	RDS bool
	// EKS enables AWS EKS sync.
	EKS bool
	// S3 enables AWS S3 sync.
	S3 bool
}

// BuildFeatures builds the feature flags based on supported types returned by Access Graph
// AWS endpoints.
func BuildFeatures(values ...string) Features {
	features := Features{}
	for _, value := range values {
		switch value {
		case awsGroups:
			features.Groups = true
		case awsUsers:
			features.Users = true
		case awsEC2:
			features.EC2 = true
		case awsEKS:
			features.EKS = true
		case awsRDS:
			features.RDS = true
		case awsS3:
			features.S3 = true
		case awsRoles:
			features.Roles = true
		}
	}
	return features
}
