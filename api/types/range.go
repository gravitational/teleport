package types

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/header"
)

type RangeScan struct {
	header.ResourceHeader
	Spec RangeScanSpec
}

type RangeScanSpec struct {
	From time.Time `json:"from,omitempty"`
}

func NewRangeScan(metadata header.Metadata, spec RangeScanSpec) (*RangeScan, error) {
	item := &RangeScan{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}
	if err := item.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return item, nil
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *RangeScan) CheckAndSetDefaults() error {
	a.SetKind(KindAccessMonitoringRangeScan)
	a.SetVersion(V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
