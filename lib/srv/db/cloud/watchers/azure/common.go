package azure

// import (
// 	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
// 	"github.com/gravitational/teleport/lib/srv/db/common"
// 	"github.com/gravitational/trace"
// )

// func serverFromAzureMySQLServer(s *armmysql.Server) (*common.AzureServer, error) {
// 	if s == nil {
// 		return nil, trace.BadParameter("nil server")
// 	}
// 	if s.Name == nil {
// 		return nil, trace.BadParameter("nil server name")
// 	}
// 	name := *s.Name

// 	if s.Type == nil {
// 		return nil, trace.BadParameter("nil server type")
// 	}
// 	serverType := *s.Type

// 	if s.Location == nil {
// 		return nil, trace.BadParameter("nil server location")
// 	}
// 	region := *s.Location

// 	if s.ID == nil {
// 		return nil, trace.BadParameter("nil server ID")
// 	}
// 	id := *s.ID

// 	tags := make(map[string]string, len(s.Tags))
// 	for k, v := range s.Tags {
// 		if v != nil {
// 			tags[k] = *v
// 		}
// 	}

// 	if s.Properties == nil {
// 		return nil, trace.BadParameter("nil server properties")
// 	}
// 	if s.Properties.Version == nil {
// 		return nil, trace.BadParameter("nil server version")
// 	}
// 	version := *s.Properties.Version

// 	if s.Properties.UserVisibleState == nil {
// 		return nil, trace.BadParameter("nil server state")
// 	}
// 	state := *s.Properties.UserVisibleState

// 	if s.Properties.FullyQualifiedDomainName == nil {
// 		return nil, trace.BadParameter("nil server endpoint")
// 	}
// 	fqdn := *s.Properties.FullyQualifiedDomainName
// 	out := &common.AzureServer{
// 		Name:    name,
// 		Type:    serverType,
// 		Region:  region,
// 		Version: string(version),
// 		State:   string(state),
// 		FQDN:    fqdn,
// 		ID:      id,
// 		Tags:    tags,
// 	}
// 	return out, nil
// }

func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
