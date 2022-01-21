/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package web

import (
	"crypto/tls"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/desktop"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
)

// GET /webapi/sites/:site/desktops/:desktopName/connect?access_token=<bearer_token>&username=<username>&width=<width>&height=<height>
func (h *Handler) desktopConnectHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
	site reversetunnel.RemoteSite,
) (interface{}, error) {
	desktopName := p.ByName("desktopName")
	if desktopName == "" {
		return nil, trace.BadParameter("missing desktopName in request URL")
	}

	log := ctx.log.WithField("desktop-name", desktopName)
	log.Debug("New desktop access websocket connection")

	if err := createDesktopConnection(w, r, desktopName, log, ctx, site); err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

func createDesktopConnection(
	w http.ResponseWriter,
	r *http.Request,
	desktopName string,
	log *logrus.Entry,
	ctx *SessionContext,
	site reversetunnel.RemoteSite,
) error {

	q := r.URL.Query()
	username := q.Get("username")
	if username == "" {
		return trace.BadParameter("missing username")
	}
	width, err := strconv.Atoi(q.Get("width"))
	if err != nil {
		return trace.BadParameter("width missing or invalid")
	}
	height, err := strconv.Atoi(q.Get("height"))
	if err != nil {
		return trace.BadParameter("height missing or invalid")
	}

	log.Debugf("Attempting to connect to desktop using username=%v, width=%v, height=%v\n", username, width, height)

	winServices, err := ctx.unsafeCachedAuthClient.GetWindowsDesktopServices(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(awly): trusted cluster support - if this request is for a different
	// cluster, dial their proxy and forward the websocket request as is.

	if len(winServices) == 0 {
		return trace.NotFound("No windows_desktop_services are registered in this cluster")
	}
	// Pick a random Windows desktop service as our gateway.
	// When agent mode is implemented in the service, we'll have to filter out
	// the services in agent mode.
	//
	// In the future, we may want to do something smarter like latency-based
	// routing.
	service := winServices[rand.Intn(len(winServices))]
	log = log.WithField("windows-service-uuid", service.GetName())
	log = log.WithField("windows-service-addr", service.GetAddr())
	serviceCon, err := site.DialTCP(reversetunnel.DialParams{
		From:     &utils.NetAddr{AddrNetwork: "tcp", Addr: r.RemoteAddr},
		To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: service.GetAddr()},
		ConnType: types.WindowsDesktopTunnel,
		ServerID: service.GetName() + "." + ctx.parent.clusterName,
	})
	if err != nil {
		return trace.WrapWithMessage(err, "failed to connect to windows_desktop_service at %q: %v", service.GetAddr(), err)
	}
	defer serviceCon.Close()
	tlsConfig := ctx.clt.Config()
	// Pass target desktop UUID via SNI.
	tlsConfig.ServerName = desktopName + desktop.SNISuffix
	serviceConTLS := tls.Client(serviceCon, ctx.clt.Config())
	log.Debug("Connected to windows_desktop_service")

	tdpConn := tdp.NewConn(serviceConTLS)
	err = tdpConn.OutputMessage(tdp.ClientUsername{Username: username})
	if err != nil {
		return trace.Wrap(err)
	}
	err = tdpConn.OutputMessage(tdp.ClientScreenSpec{Width: uint32(width), Height: uint32(height)})
	if err != nil {
		return trace.Wrap(err)
	}

	websocket.Handler(func(conn *websocket.Conn) {
		if err := proxyWebsocketConn(conn, serviceConTLS); err != nil {
			log.WithError(err).Warningf("Error proxying a desktop protocol websocket to windows_desktop_service")
		}
	}).ServeHTTP(w, r)
	return nil
}

func proxyWebsocketConn(ws *websocket.Conn, con net.Conn) error {
	// Ensure we send binary frames to the browser.
	ws.PayloadType = websocket.BinaryFrame

	// TODO(zmb3): improve graceful shutdown handling
	// (if the browser intentionally disconnects, we don't want to use "use of closed connection" errors)

	errs := make(chan error, 2)
	go func() {
		defer ws.Close()
		defer con.Close()

		_, err := io.Copy(ws, con)
		errs <- err
	}()
	go func() {
		defer ws.Close()
		defer con.Close()

		_, err := io.Copy(con, ws)
		errs <- err
	}()

	var retErrs []error
	for i := 0; i < 2; i++ {
		retErrs = append(retErrs, <-errs)
	}
	return trace.NewAggregate(retErrs...)
}

// playbackAction is a message passed from the playback client
// to the server over the websocket connection.
type playbackAction struct {
	// Action is one of "play" | "pause"
	Action string `json:"action"`
}

// playbackState is a thread-safe struct for managing
// the global state of the playback websocket connection.
type playbackState struct {
	ws      *websocket.Conn
	playing bool
	wsOpen  bool
	mu      sync.RWMutex
}

// GetWsOpen gets the wsOpen value. It should be called at the top of continuously
// looping goroutines that use playbackState, who should return if it returns false.
func (ps *playbackState) GetWsOpen() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.wsOpen
}

// Close closes the websocket connection and sets wsOpen to false.
// It should be deferred by all the goroutines that use playbackState,
// in order to ensure that when one goroutine closes, all the others do too
// (when used in conjunction with GetWsOpen).
func (ps *playbackState) Close() {
	ps.ws.Close()
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.wsOpen = false
}

func (h *Handler) desktopPlaybackHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
) (interface{}, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing sid in request URL")
	}

	websocket.Handler(func(ws *websocket.Conn) {
		rCtx := r.Context()
		ws.PayloadType = websocket.BinaryFrame
		ps := playbackState{
			ws:      ws,
			playing: true,
			wsOpen:  true,
		}
		defer ps.Close()
		defer h.log.Debug("playback websocket closed")

		// Handle incoming playback actions.
		go func() {
			defer ps.Close()
			defer h.log.Debug("playback action-recieving goroutine returned")
			for {
				if !ps.GetWsOpen() {
					return
				}

				action := playbackAction{}
				err := websocket.JSON.Receive(ws, &action)
				if err != nil {
					h.log.WithError(err).Error("error reading from websocket")
					return
				}
				if action.Action != "" {
					h.log.Debugf("recieved playback action: %+v", action)
					// TODO
				}
			}
		}()

		// Stream session events back to browser.
		go func() {
			defer ps.Close()
			defer h.log.Debug("playback event streaming goroutine returned")
			var lastDelay int64
			eventsC, errC := ctx.clt.StreamSessionEvents(rCtx, session.ID(sID), 0)
			for {
				if !ps.GetWsOpen() {
					return
				}

				select {
				case err := <-errC:
					h.log.WithError(err).Errorf("streaming session %v", sID)
					return
				case evt := <-eventsC:
					if evt == nil {
						h.log.Debug("reached end of playback, restarting")
						lastDelay = 0
						// TODO: this causes the session to be re-downloaded, make it smarter by checking for the file first?
						eventsC, errC = ctx.clt.StreamSessionEvents(rCtx, session.ID(sID), 0)
						continue
					}
					switch e := evt.(type) {
					case *apievents.DesktopRecording:
						if e.DelayMilliseconds > lastDelay {
							time.Sleep(time.Duration(e.DelayMilliseconds-lastDelay) * time.Millisecond)
							lastDelay = e.DelayMilliseconds
						}
						if _, err := ws.Write(e.Message); err != nil {
							h.log.WithError(err).Error("failed to write TDP message over websocket")
							return
						}
					default:
						h.log.Warnf("session %v contains unexpected event type %T", sID, evt)
					}
				default:
					continue
				}
			}
		}()

		// Hang until the request context is cancelled or the websocket is closed by another goroutine.
		for {
			// Return if websocket is closed by another goroutine calling ps.Close()
			if !ps.GetWsOpen() {
				return
			}

			// Return if websocket is closed by the browser.
			select {
			case <-rCtx.Done():
				return
			default:
				continue
			}
		}

	}).ServeHTTP(w, r)
	return nil, nil
}
