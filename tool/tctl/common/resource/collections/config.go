package collections

import (
	"fmt"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
	"io"
	"strconv"
	"strings"
)

func NewAuthPreferenceCollection(pref types.AuthPreference) ResourceCollection {
	return &authPrefCollection{authPref: pref}
}

type authPrefCollection struct {
	authPref types.AuthPreference
}

func (c *authPrefCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.authPref}
}

func (c *authPrefCollection) WriteText(w io.Writer, verbose bool) error {
	var secondFactorStrings []string
	for _, sf := range c.authPref.GetSecondFactors() {
		sfString, err := sf.Encode()
		if err != nil {
			return trace.Wrap(err)
		}
		secondFactorStrings = append(secondFactorStrings, sfString)
	}

	t := asciitable.MakeTable([]string{"Type", "Second Factors"})
	t.AddRow([]string{c.authPref.GetType(), strings.Join(secondFactorStrings, ", ")})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewUIConfigCollection(config types.UIConfig) ResourceCollection {
	return &uiConfigCollection{uiconfig: config}
}

type uiConfigCollection struct {
	uiconfig types.UIConfig
}

func (c *uiConfigCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.uiconfig}
}

func (c *uiConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Scrollback Lines", "Show Resources"})
	t.AddRow([]string{strconv.FormatInt(int64(c.uiconfig.GetScrollbackLines()), 10), string(c.uiconfig.GetShowResources())})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewNetworkConfigCollection(config types.ClusterNetworkingConfig) ResourceCollection {
	return &netConfigCollection{netConfig: config}
}

type netConfigCollection struct {
	netConfig types.ClusterNetworkingConfig
}

func (c *netConfigCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.netConfig}
}

func (c *netConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Client Idle Timeout", "Keep Alive Interval", "Keep Alive Count Max", "Session Control Timeout"})
	t.AddRow([]string{
		c.netConfig.GetClientIdleTimeout().String(),
		c.netConfig.GetKeepAliveInterval().String(),
		strconv.FormatInt(c.netConfig.GetKeepAliveCountMax(), 10),
		c.netConfig.GetSessionControlTimeout().String(),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewMaintenanceWindowCollection(cmc types.ClusterMaintenanceConfig) ResourceCollection {
	return &maintenanceWindowCollection{cmc: cmc}
}

type maintenanceWindowCollection struct {
	cmc types.ClusterMaintenanceConfig
}

func (c *maintenanceWindowCollection) Resources() (r []types.Resource) {
	if c.cmc == nil {
		return nil
	}
	return []types.Resource{c.cmc}
}

func (c *maintenanceWindowCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Type", "Params"})

	agentUpgradeParams := "none"

	if c.cmc != nil {
		if win, ok := c.cmc.GetAgentUpgradeWindow(); ok {
			agentUpgradeParams = fmt.Sprintf("utc_start_hour=%d", win.UTCStartHour)
			if len(win.Weekdays) != 0 {
				agentUpgradeParams = fmt.Sprintf("%s, weekdays=%s", agentUpgradeParams, strings.Join(win.Weekdays, ","))
			}
		}
	}

	t.AddRow([]string{"Agent Upgrades", agentUpgradeParams})

	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewRecConfigCollection(config types.SessionRecordingConfig) ResourceCollection {
	return &recConfigCollection{recConfig: config}
}

type recConfigCollection struct {
	recConfig types.SessionRecordingConfig
}

func (c *recConfigCollection) Resources() (r []types.Resource) {
	return []types.Resource{c.recConfig}
}

func (c *recConfigCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Mode", "Proxy Checks Host Keys"})
	t.AddRow([]string{c.recConfig.GetMode(), strconv.FormatBool(c.recConfig.GetProxyChecksHostKeys())})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func NewNetworkRestrictionCollection(restrictions types.NetworkRestrictions) ResourceCollection {
	return &netRestrictionsCollection{netRestricts: restrictions}
}

type netRestrictionsCollection struct {
	netRestricts types.NetworkRestrictions
}

type writer struct {
	w   io.Writer
	err error
}

func (w *writer) write(s string) {
	if w.err == nil {
		_, w.err = w.w.Write([]byte(s))
	}
}

func (c *netRestrictionsCollection) Resources() (r []types.Resource) {
	r = append(r, c.netRestricts)
	return
}

func (c *netRestrictionsCollection) writeList(as []types.AddressCondition, w *writer) {
	for _, a := range as {
		w.write(a.CIDR)
		w.write("\n")
	}
}

func (c *netRestrictionsCollection) WriteText(w io.Writer, verbose bool) error {
	out := &writer{w: w}
	out.write("ALLOW\n")
	c.writeList(c.netRestricts.GetAllow(), out)

	out.write("\nDENY\n")
	c.writeList(c.netRestricts.GetDeny(), out)
	return trace.Wrap(out.err)
}
