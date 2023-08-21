package ui

import "github.com/gravitational/teleport/api/types/accesslist"

type AccessListResponse struct {
	AccessLists []*accesslist.AccessList `json:"accessLists,omitempty"`
	AccessList  *accesslist.AccessList   `json:"accessList,omitempty"`
}

type CreateAccessListRequest struct {
	AuditDuration string `json:"auditDuration"`
	accesslist.Spec
}
