// Package state is a minimal package for the Teleport operator to store data in Kubernetes.
// It supports running multiple operator replicas.
// Content stored should be kept to a strict minimum.
// Currently, the only content stored is the operator ID.
// See RFD 324 for more details.
package state
