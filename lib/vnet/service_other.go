package vnet

import "context"

type ServiceStatus struct {
	Installed bool
	Version   string
}

func GetServiceStatus(ctx context.Context) (ServiceStatus, error) {
	return ServiceStatus{}, nil
}
