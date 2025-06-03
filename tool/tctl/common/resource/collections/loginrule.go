package collections

import (
	"io"
	"strconv"

	"github.com/gravitational/trace"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/tool/tctl/common/loginrule"
)

type loginRuleCollection struct {
	rules []*loginrulepb.LoginRule
}

func NewLoginRuleCollection(rules []*loginrulepb.LoginRule) ResourceCollection {
	return &loginRuleCollection{rules: rules}
}

func (l *loginRuleCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Priority"})
	for _, rule := range l.rules {
		t.AddRow([]string{rule.Metadata.Name, strconv.FormatInt(int64(rule.Priority), 10)})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (l *loginRuleCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(l.rules))
	for i, rule := range l.rules {
		resources[i] = loginrule.ProtoToResource(rule)
	}
	return resources
}
