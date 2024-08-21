package okta

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/types"
)

type nullStatusUpdate struct{}

func (nullStatusUpdate) SetCode(context.Context, types.PluginStatusCode) {
}

func (nullStatusUpdate) UpdateUserSync(context.Context, time.Time /* nUsers */, int, error) {

}

func (nullStatusUpdate) UpdateAppGroupSync(context.Context, time.Time /* nApps*/, int /* nGroups */, int, error) {
}

func (nullStatusUpdate) UpdateAccessListSync(context.Context, time.Time /* nApps */, int /* nGroups */, int, error) {
}
