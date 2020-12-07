package types

import fmt "fmt"

// String represents a human readable version of remote cluster settings.
func (c *RemoteClusterV3) String() string {
	return fmt.Sprintf("RemoteCluster(%v, %v)", c.Metadata.Name, c.Status.Connection)
}
