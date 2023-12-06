//go:build bpf && !386
// +build bpf,!386

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package restrictedsession

import (
	"bytes"
	"embed"
	"encoding/binary"
	"os"
	"sync"
	"unsafe"

	"github.com/aquasecurity/libbpfgo"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentRestrictedSession,
})

var (
	lostRestrictedEvents = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricLostRestrictedEvents,
			Help: "Number of lost restricted events.",
		},
	)
)

//go:embed bytecode
var embedFS embed.FS

func init() {
	prometheus.MustRegister(lostRestrictedEvents)
}

var unit = make([]byte, 1)

// sessionMgr implements restrctedsession.Manager interface
// by enforcing the rules via LSM BPF hooks
type sessionMgr struct {
	// mod is the handle to the BPF loaded module
	mod *libbpfgo.Module

	// watch keeps the set of cgroups being enforced
	watch bpf.SessionWatch

	// cgroups for which enforcement is active
	restrictedCGroups *libbpfgo.BPFMap

	// network blocking subsystem
	nw *network

	// eventLoop pumps the audit messages from the kernel
	// to the audit subsystem
	eventLoop *auditEventLoop

	// updateLoop listens for restriction updates and applies them
	// to the audit subsystem
	updateLoop *restrictionsUpdateLoop
}

// New creates a RestrictedSession service.
func New(config *servicecfg.RestrictedSessionConfig, wc RestrictionsWatcherClient) (Manager, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If BPF-based auditing is not enabled, don't configure anything
	// right away.
	if !config.Enabled {
		log.Debugf("Restricted session is not enabled, skipping.")
		return &NOP{}, nil
	}

	// Before proceeding, check that eBPF based LSM is enabled in the kernel
	if err = checkBpfLsm(); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("Starting restricted session.")

	restrictedBPF, err := embedFS.ReadFile("bytecode/restricted.bpf.o")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mod, err := libbpfgo.NewModuleFromBuffer(restrictedBPF, "restricted")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Load into the kernel
	if err = mod.BPFLoadObject(); err != nil {
		return nil, trace.Wrap(err)
	}

	nw, err := newNetwork(mod)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cgroups, err := mod.GetMap("restricted_cgroups")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	m := &sessionMgr{
		mod:               mod,
		watch:             bpf.NewSessionWatch(),
		restrictedCGroups: cgroups,
		nw:                nw,
	}

	m.eventLoop, err = newAuditEventLoop(mod, &m.watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	m.updateLoop, err = newRestrictionsUpdateLoop(nw, wc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Info("Started restricted session management")

	return m, nil
}

// Close will stop any running BPF programs. Note this is only for a graceful
// shutdown, from the man page for BPF: "Generally, eBPF programs are loaded
// by the user process and automatically unloaded when the process exits."
func (m *sessionMgr) Close() {
	// Close the updater loop
	m.updateLoop.close()

	// Signal the loop pulling events off the perf buffer to shutdown.
	m.eventLoop.close()
}

// OpenSession inserts the cgroupID into the BPF hash map to enable
// enforcement by the kernel
func (m *sessionMgr) OpenSession(ctx *bpf.SessionContext, cgroupID uint64) {
	m.watch.Add(cgroupID, ctx)

	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, cgroupID)

	m.restrictedCGroups.Update(unsafe.Pointer(&key[0]), unsafe.Pointer(&unit[0]))

	log.Debugf("CGroup %v registered", cgroupID)
}

// CloseSession removes the cgroupID from the BPF hash map to enable
// enforcement by the kernel
func (m *sessionMgr) CloseSession(ctx *bpf.SessionContext, cgroupID uint64) {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, cgroupID)

	m.restrictedCGroups.DeleteKey(unsafe.Pointer(&key[0]))

	m.watch.Remove(cgroupID)

	log.Debugf("CGroup %v unregistered", cgroupID)
}

type restrictionsUpdateLoop struct {
	nw *network

	watcher *RestrictionsWatcher

	// Notifies that loop goroutine is done
	wg sync.WaitGroup
}

func newRestrictionsUpdateLoop(nw *network, wc RestrictionsWatcherClient) (*restrictionsUpdateLoop, error) {
	w, err := NewRestrictionsWatcher(RestrictionsWatcherConfig{
		Client:        wc,
		RestrictionsC: make(chan *NetworkRestrictions, 10),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	l := &restrictionsUpdateLoop{
		nw:      nw,
		watcher: w,
	}

	l.wg.Add(1)
	go l.loop()

	return l, nil
}

func (l *restrictionsUpdateLoop) close() {
	l.watcher.Close()
	l.wg.Wait()
}

func (l *restrictionsUpdateLoop) loop() {
	defer l.wg.Done()

	for r := range l.watcher.RestrictionsC {
		l.nw.update(r)
	}
}

type auditEventLoop struct {
	// Maps the cgroup to the session
	watch *bpf.SessionWatch

	// BPF ring buffer for reported audit (blocked) events
	events *bpf.RingBuffer

	// Keeps track of the number of lost audit events
	lost *bpf.Counter

	// Notifies that loop goroutine is done
	wg sync.WaitGroup
}

// loop pulls events off the perf ring buffer, parses them, and emits them to
// the audit log.
func newAuditEventLoop(mod *libbpfgo.Module, w *bpf.SessionWatch) (*auditEventLoop, error) {
	events, err := bpf.NewRingBuffer(mod, "audit_events")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lost, err := bpf.NewCounter(mod, "lost", lostRestrictedEvents)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	l := &auditEventLoop{
		watch:  w,
		events: events,
		lost:   lost,
	}

	l.wg.Add(1)
	go l.loop()

	return l, nil
}

func (l *auditEventLoop) loop() {
	defer l.wg.Done()

	for eventBytes := range l.events.EventCh {
		buf := bytes.NewBuffer(eventBytes)
		hdr, err := parseAuditEventHeader(buf)
		if err != nil {
			log.Error(err.Error())
			continue
		}

		ctx, ok := l.watch.Get(hdr.CGroupID)
		if !ok {
			log.Errorf("Blocked event for unknown cgroup ID (%v)", hdr.CGroupID)
			continue
		}

		event, err := parseAuditEvent(buf, &hdr, ctx)
		if err != nil {
			log.WithError(err).Error("Failed to parse network event.")
			continue
		}

		if err = ctx.Emitter.EmitAuditEvent(ctx.Context, event); err != nil {
			log.WithError(err).Warn("Failed to emit network event.")
		}
	}
}

func (l *auditEventLoop) close() {
	// Cleanup
	l.events.Close()
	l.lost.Close()

	l.wg.Wait()
}

// checkBpfLsm checks that eBPF is one of the enabled
// LSM "modules".
func checkBpfLsm() error {
	const lsmInfo = "/sys/kernel/security/lsm"

	csv, err := os.ReadFile(lsmInfo)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, mod := range bytes.Split(csv, []byte(",")) {
		if bytes.Equal(mod, []byte("bpf")) {
			return nil
		}
	}

	return trace.Errorf(`%s does not contain bpf entry, indicating that the kernel
is not enabled for eBPF based LSM enforcement. Make sure the kernel is compiled with
CONFIG_BPF_LSM=y and enabled via CONFIG_LSM or lsm= boot option`, lsmInfo)
}

// attachLSM attaches the LSM programs in the module to
// kernel hook points.
func attachLSM(mod *libbpfgo.Module, name string) error {
	prog, err := mod.GetProgram(name)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = prog.AttachLSM()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
