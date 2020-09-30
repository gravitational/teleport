/*
Copyright 2020 Gravitational, Inc.

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

package events

import (
	"context"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// UploadCompleterConfig specifies configuration for the uploader
type UploadCompleterConfig struct {
	// Uploader allows the completer to list and complete uploads
	Uploader MultipartUploader
	// GracePeriod is the period after which uploads are considered
	// abandoned and will be completed
	GracePeriod time.Duration
	// Component is a component used in logging
	Component string
	// CheckPeriod is a period for checking the upload
	CheckPeriod time.Duration
	// Clock is used to override clock in tests
	Clock clockwork.Clock
	// Unstarted does not start automatic goroutine,
	// is useful when completer is embedded in another function
	Unstarted bool
}

// CheckAndSetDefaults checks and sets default values
func (cfg *UploadCompleterConfig) CheckAndSetDefaults() error {
	if cfg.Uploader == nil {
		return trace.BadParameter("missing parameter Uploader")
	}
	if cfg.GracePeriod == 0 {
		cfg.GracePeriod = defaults.UploadGracePeriod
	}
	if cfg.Component == "" {
		cfg.Component = teleport.ComponentAuth
	}
	if cfg.CheckPeriod == 0 {
		cfg.CheckPeriod = defaults.LowResPollingPeriod
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewUploadCompleter returns a new instance of the upload completer
// the completer has to be closed to release resources and goroutines
func NewUploadCompleter(cfg UploadCompleterConfig) (*UploadCompleter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	u := &UploadCompleter{
		cfg: cfg,
		log: log.WithFields(log.Fields{
			trace.Component: teleport.Component(cfg.Component, "completer"),
		}),
		cancel:   cancel,
		closeCtx: ctx,
	}
	if !cfg.Unstarted {
		go u.run()
	}
	return u, nil
}

// UploadCompleter periodically scans uploads that have not been completed
// and completes them
type UploadCompleter struct {
	cfg      UploadCompleterConfig
	log      *log.Entry
	cancel   context.CancelFunc
	closeCtx context.Context
}

func (u *UploadCompleter) run() {
	ticker := u.cfg.Clock.NewTicker(u.cfg.CheckPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			if err := u.CheckUploads(u.closeCtx); err != nil {
				u.log.WithError(err).Warningf("Failed to check uploads.")
			}
		case <-u.closeCtx.Done():
			return
		}
	}
}

// CheckUploads fetches uploads, checks if any uploads exceed grace period
// and completes unfinished uploads
func (u *UploadCompleter) CheckUploads(ctx context.Context) error {
	uploads, err := u.cfg.Uploader.ListUploads(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	u.log.Debugf("Got %v active uploads.", len(uploads))
	for _, upload := range uploads {
		gracePoint := upload.Initiated.Add(u.cfg.GracePeriod)
		if !gracePoint.Before(u.cfg.Clock.Now()) {
			return nil
		}
		parts, err := u.cfg.Uploader.ListParts(ctx, upload)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(parts) == 0 {
			continue
		}
		u.log.Debugf("Upload %v grace period is over. Trying complete.", upload)
		if err := u.cfg.Uploader.CompleteUpload(ctx, upload, parts); err != nil {
			return trace.Wrap(err)
		}
		u.log.Debugf("Completed upload %v.", upload)
	}
	return nil
}

// Close closes all outstanding operations without waiting
func (u *UploadCompleter) Close() error {
	u.cancel()
	return nil
}
