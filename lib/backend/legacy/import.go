package legacy

import (
	"context"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Importer specifies methods for importing data
// from legacy backends
type Importer interface {
	// Import imports elements, makes sure elements are imported only once
	// returns trace.AlreadyExists if elements have been imported
	Import(ctx context.Context, items []backend.Item) error
	// Imported returns true if backend already imported data from another backend
	Imported(ctx context.Context) (bool, error)
	// Close closes importer
	Close() error
}

type Exporter interface {
	// Export exports all items from the backend in new backend Items
	Export() ([]backend.Item, error)
	// Close closes importer
	Close() error
}

// NewExporterFunc returns new exporter
type NewExporterFunc func() (Exporter, error)

// Import imports backend data into importer unless importer has already
// imported data. If Importer has no imported data yet, exporter will
// not be initialized. This function can be called many times on the
// same importer. Importer will be closed if import has failed.
func Import(ctx context.Context, importer Importer, newExporter NewExporterFunc) error {
	log := logrus.WithFields(logrus.Fields{
		trace.Component: teleport.Component(teleport.ComponentMigrate),
	})
	err := func() error {
		imported, err := importer.Imported(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		if imported {
			log.Debugf("Detected legacy backend, data has already been imported.")
			return nil
		}
		log.Infof("Importing data from legacy backend.")
		exporter, err := newExporter()
		if err != nil {
			return trace.Wrap(err)
		}
		defer exporter.Close()
		items, err := exporter.Export()
		if err != nil {
			return trace.Wrap(err)
		}
		if err := importer.Import(ctx, items); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Successfully imported %v items.", len(items))
		return nil
	}()
	if err != nil {
		if err := importer.Close(); err != nil {
			log.Errorf("Failed to close backend: %v.", err)
		}
	}
	return trace.Wrap(err)
}
