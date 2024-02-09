// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dynamoathenamigration

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/athena"
)

type Config struct {
	// ExportTime is time in the past from which to export table data.
	ExportTime time.Time

	// ExportARN allows to use already finished export without triggering new.
	ExportARN string

	// DynamoTableARN that will be exported.
	DynamoTableARN string

	// ExportLocalDir specifies where export files will be downloaded (it must exists).
	// If empty os.TempDir() will be used.
	ExportLocalDir string

	// MaxMemoryUsedForSortingExportInMB (MB) is used to define how large amount of events
	// will be loaded into memory when doing sorting of events before publishing it.
	MaxMemoryUsedForSortingExportInMB int

	// Bucket used to store export.
	Bucket string
	// Prefix is s3 prefix where to store export inside bucket.
	Prefix string

	// DryRun allows to generate export and convert it to AuditEvents.
	// Nothing is published to athena publisher.
	// Can be used to test if export is valid.
	DryRun bool

	// NoOfEmitWorkers defines how many workers are used to emit audit events.
	NoOfEmitWorkers int
	bufferSize      int

	// CheckpointPath is full path of file where checkpoint data should be stored.
	// Defaults to file in current directory (athenadynamomigration.json)
	// Checkpoint allow to resume export which failed during emitting.
	CheckpointPath string

	// TopicARN is topic of athena logger.
	TopicARN string
	// LargePayloadBucket is s3 bucket configured for large payloads in athena logger.
	LargePayloadBucket string
	// LargePayloadPrefix is s3 prefix configured for large payloads in athena logger.
	LargePayloadPrefix string

	Logger log.FieldLogger
}

const (
	defaultCheckpointPath                    = "athenadynamomigration.json"
	DefaultMaxMemoryUsedForSortingExportInMB = 500
)

func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.ExportTime.IsZero() {
		cfg.ExportTime = time.Now()
	}
	if cfg.DynamoTableARN == "" && cfg.ExportARN == "" {
		return trace.BadParameter("either DynamoTableARN or ExportARN is required")
	}
	if cfg.Bucket == "" {
		return trace.BadParameter("missing export bucket")
	}
	if cfg.NoOfEmitWorkers == 0 {
		cfg.NoOfEmitWorkers = 3
	}
	if cfg.bufferSize == 0 {
		cfg.bufferSize = 10 * cfg.NoOfEmitWorkers
	}
	if cfg.MaxMemoryUsedForSortingExportInMB == 0 {
		cfg.MaxMemoryUsedForSortingExportInMB = DefaultMaxMemoryUsedForSortingExportInMB
	}
	if !cfg.DryRun {
		if cfg.TopicARN == "" {
			return trace.BadParameter("missing Athena logger SNS Topic ARN")
		}

		if cfg.LargePayloadBucket == "" {
			return trace.BadParameter("missing Athena logger large payload bucket")
		}
	}
	if cfg.CheckpointPath == "" {
		cfg.CheckpointPath = defaultCheckpointPath
	}

	if cfg.Logger == nil {
		cfg.Logger = log.New()
	}
	return nil
}

type task struct {
	Config
	dynamoClient  *dynamodb.Client
	s3Downloader  s3downloader
	eventsEmitter eventsEmitter
}

type s3downloader interface {
	Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*manager.Downloader)) (n int64, err error)
}

type eventsEmitter interface {
	EmitAuditEvent(ctx context.Context, in apievents.AuditEvent) error
}

func newMigrateTask(ctx context.Context, cfg Config, awsCfg aws.Config) (*task, error) {
	s3Client := s3.NewFromConfig(awsCfg)
	return &task{
		Config:       cfg,
		dynamoClient: dynamodb.NewFromConfig(awsCfg),
		s3Downloader: manager.NewDownloader(s3Client),
		eventsEmitter: athena.NewPublisher(athena.PublisherConfig{
			TopicARN: cfg.TopicARN,
			SNSPublisher: sns.NewFromConfig(awsCfg, func(o *sns.Options) {
				o.Retryer = retry.NewStandard(func(so *retry.StandardOptions) {
					so.MaxAttempts = 30
					so.MaxBackoff = 1 * time.Minute
					// Use bigger rate limit to handle default sdk throttling: https://github.com/aws/aws-sdk-go-v2/issues/1665
					so.RateLimiter = ratelimit.NewTokenRateLimit(1000000)
				})
			}),
			Uploader:      manager.NewUploader(s3Client),
			PayloadBucket: cfg.LargePayloadBucket,
			PayloadPrefix: cfg.LargePayloadPrefix,
		}),
	}, nil
}

// Migrate executed dynamodb -> athena migration.
func Migrate(ctx context.Context, cfg Config) error {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(MigrateWithAWS(ctx, cfg, awsCfg))
}

// MigrateWithAWS executed dynamodb -> athena migration. Provide your own awsCfg
func MigrateWithAWS(ctx context.Context, cfg Config, awsCfg aws.Config) error {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	t, err := newMigrateTask(ctx, cfg, awsCfg)
	if err != nil {
		return trace.Wrap(err)
	}

	exportInfo, err := t.GetOrStartExportAndWaitForResults(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := t.ProcessDataObjects(ctx, exportInfo); err != nil {
		return trace.Wrap(err)
	}

	t.Logger.Info("Migration finished")
	return nil
}

// GetOrStartExportAndWaitForResults return export results.
// It can either reused existing export or start new one, depending on FreshnessWindow.
func (t *task) GetOrStartExportAndWaitForResults(ctx context.Context) (*exportInfo, error) {
	exportARN := t.Config.ExportARN
	if exportARN == "" {
		var err error
		exportARN, err = t.startExportJob(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	manifest, err := t.waitForCompletedExport(ctx, exportARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	t.Logger.Debugf("Using export manifest %s", manifest)
	dataObjectsInfo, err := t.getDataObjectsInfo(ctx, manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &exportInfo{
		ExportARN:       exportARN,
		DataObjectsInfo: dataObjectsInfo,
	}, nil
}

// ProcessDataObjects takes dataObjectInfo from export summary, downloads data files
// from s3, ungzip them and emitt them on SNS using athena publisher.
func (t *task) ProcessDataObjects(ctx context.Context, exportInfo *exportInfo) error {
	eventsC := make(chan apievents.AuditEvent, t.bufferSize)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		err := t.getEventsFromDataFiles(egCtx, exportInfo, eventsC)
		close(eventsC)
		return trace.Wrap(err)
	})

	eg.Go(func() error {
		err := t.emitEvents(egCtx, eventsC, exportInfo.ExportARN)
		return trace.Wrap(err)
	})

	return trace.Wrap(eg.Wait())
}

func (t *task) waitForCompletedExport(ctx context.Context, exportARN string) (exportManifest string, err error) {
	req := &dynamodb.DescribeExportInput{
		ExportArn: aws.String(exportARN),
	}
	for {
		exportStatusOutput, err := t.dynamoClient.DescribeExport(ctx, req)
		if err != nil {
			return "", trace.Wrap(err)
		}

		if exportStatusOutput == nil || exportStatusOutput.ExportDescription == nil {
			return "", errors.New("dynamo DescribeExport returned unexpected nil on response")
		}

		exportStatus := exportStatusOutput.ExportDescription.ExportStatus
		switch exportStatus {
		case dynamoTypes.ExportStatusCompleted:
			return aws.ToString(exportStatusOutput.ExportDescription.ExportManifest), nil
		case dynamoTypes.ExportStatusFailed:
			return "", trace.Errorf("export %s returned failed status", exportARN)
		case dynamoTypes.ExportStatusInProgress:
			select {
			case <-ctx.Done():
				return "", trace.Wrap(ctx.Err())
			case <-time.After(30 * time.Second):
				t.Logger.Debug("Export job still in progress...")
			}
		default:
			return "", trace.Errorf("dynamo DescribeExport returned unexpected status: %v", exportStatus)
		}

	}
}

func (t *task) startExportJob(ctx context.Context) (arn string, err error) {
	exportOutput, err := t.dynamoClient.ExportTableToPointInTime(ctx, &dynamodb.ExportTableToPointInTimeInput{
		S3Bucket:     aws.String(t.Bucket),
		TableArn:     aws.String(t.DynamoTableARN),
		ExportFormat: dynamoTypes.ExportFormatDynamodbJson,
		ExportTime:   aws.Time(t.ExportTime),
		S3Prefix:     aws.String(t.Prefix),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	if exportOutput == nil || exportOutput.ExportDescription == nil {
		return "", errors.New("dynamo ExportTableToPointInTime returned unexpected nil on response")
	}

	exportArn := aws.ToString(exportOutput.ExportDescription.ExportArn)
	t.Logger.Infof("Started export %s", exportArn)
	return exportArn, nil
}

type exportInfo struct {
	ExportARN       string
	DataObjectsInfo []dataObjectInfo
}

type dataObjectInfo struct {
	DataFileS3Key string `json:"dataFileS3Key"`
	ItemCount     int    `json:"itemCount"`
}

// getDataObjectsInfo downloads manifest-files.json and get data object info from it.
func (t *task) getDataObjectsInfo(ctx context.Context, manifestPath string) ([]dataObjectInfo, error) {
	// summary file is small, we can use in-memory buffer.
	writeAtBuf := manager.NewWriteAtBuffer([]byte{})
	if _, err := t.s3Downloader.Download(ctx, writeAtBuf, &s3.GetObjectInput{
		Bucket: aws.String(t.Bucket),
		// AWS SDK returns manifest-summary.json path. We are interested in
		// manifest-files.json because it's contains references about data export files.
		Key: aws.String(path.Dir(manifestPath) + "/manifest-files.json"),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	var out []dataObjectInfo
	scanner := bufio.NewScanner(bytes.NewBuffer(writeAtBuf.Bytes()))
	// manifest-files are JSON lines files, that why it's scanned line by line.
	for scanner.Scan() {
		var obj dataObjectInfo
		err := json.Unmarshal(scanner.Bytes(), &obj)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, obj)
	}
	if err := scanner.Err(); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

func (t *task) getEventsFromDataFiles(ctx context.Context, exportInfo *exportInfo, eventsC chan<- apievents.AuditEvent) error {
	checkpoint, err := t.loadEmitterCheckpoint(ctx, exportInfo.ExportARN)
	if err != nil {
		return trace.Wrap(err)
	}

	if checkpoint != nil {
		if checkpoint.FinishedWithError {
			reuse, err := prompt.Confirmation(ctx, os.Stdout, prompt.Stdin(), fmt.Sprintf("It seems that previous migration %s stopped with error, do you want to resume it?", exportInfo.ExportARN))
			if err != nil {
				return trace.Wrap(err)
			}
			if reuse {
				t.Logger.Info("Resuming emitting from checkpoint")
			} else {
				// selected not reuse
				checkpoint = nil
			}
		} else {
			// migration completed without any error, no sense of reusing checkpoint.
			t.Logger.Info("Skipping checkpoint because previous migration finished without error")
			checkpoint = nil
		}
	}

	// afterCheckpoint is used to pass information between fromS3ToChan calls
	// if checkpoint was reached.
	var afterCheckpoint bool
	for _, dataObj := range exportInfo.DataObjectsInfo {
		t.Logger.Debugf("Downloading %s", dataObj.DataFileS3Key)
		afterCheckpoint, err = t.fromS3ToChan(ctx, dataObj, eventsC, checkpoint, afterCheckpoint)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (t *task) fromS3ToChan(ctx context.Context, dataObj dataObjectInfo, eventsC chan<- apievents.AuditEvent, checkpoint *checkpointData, afterCheckpointIn bool) (afterCheckpointOut bool, err error) {
	sortedExportFile, err := t.downloadFromS3AndSort(ctx, dataObj)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer sortedExportFile.Close()

	checkpointValues := checkpoint.checkpointValues()
	afterCheckpoint := afterCheckpointIn

	t.Logger.Debugf("Scanning %d events", dataObj.ItemCount)
	count := 0
	decoder := json.NewDecoder(sortedExportFile)
	for decoder.More() {
		count++
		ev, err := exportedDynamoItemToAuditEvent(ctx, decoder)
		if err != nil {
			return false, trace.Wrap(err)
		}

		if ev.GetID() == "" || ev.GetID() == uuid.Nil.String() {
			ev.SetID(uuid.NewString())
		}
		// Typically there should not be event without time, however it happen
		// in past due to some bugs. We decided it's better to keep in athena
		// then drop it. 1970-01-01 is used to provide valid unix timestamp.
		if ev.GetTime().IsZero() {
			ev.SetTime(time.Unix(0, 0))
		}

		// if checkpoint is present, it means that previous run ended with error
		// and we want to continue from last valid checkpoint.
		// We have list of checkpoints because processing is done in async way with
		// multiple workers. We are looking for first id among checkpoints.
		if checkpoint != nil && !afterCheckpoint {
			if !slices.Contains(checkpointValues, ev.GetID()) {
				// skipping because was processed in previous run.
				continue
			} else {
				t.Logger.Debugf("Event %s is last checkpoint, will start emitting from next event on the list", ev.GetID())
				// id is on list of valid checkpoints
				afterCheckpoint = true
				// This was last completed, skip it and from next iteration emit everything.
				continue
			}
		}

		select {
		case eventsC <- ev:
		case <-ctx.Done():
			return false, ctx.Err()
		}

		if count%1000 == 0 && !t.DryRun {
			t.Logger.Debugf("Sent on buffer %d/%d events from %s", count, dataObj.ItemCount, dataObj.DataFileS3Key)
		}
	}

	return afterCheckpoint, nil
}

// exportedDynamoItemToAuditEvent converts single line of dynamo export into AuditEvent.
func exportedDynamoItemToAuditEvent(ctx context.Context, decoder *json.Decoder) (apievents.AuditEvent, error) {
	var itemMap map[string]map[string]any
	if err := decoder.Decode(&itemMap); err != nil {
		return nil, trace.Wrap(err)
	}

	var attributeMap map[string]dynamoTypes.AttributeValue
	if err := awsAwsjson10_deserializeDocumentAttributeMap(&attributeMap, itemMap["Item"]); err != nil {
		return nil, trace.Wrap(err)
	}

	var eventFields events.EventFields
	if err := attributevalue.Unmarshal(attributeMap["FieldsMap"], &eventFields); err != nil {
		return nil, trace.Wrap(err)
	}

	event, err := events.FromEventFields(eventFields)
	return event, trace.Wrap(err)
}

func (t *task) downloadFromS3AndSort(ctx context.Context, dataObj dataObjectInfo) (*os.File, error) {
	originalName := path.Base(dataObj.DataFileS3Key)

	var dir string
	if t.Config.ExportLocalDir != "" {
		dir = t.Config.ExportLocalDir
	} else {
		dir = os.TempDir()
	}
	path := path.Join(dir, originalName)

	originalFile, err := os.Create(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := t.s3Downloader.Download(ctx, originalFile, &s3.GetObjectInput{
		Bucket: aws.String(t.Bucket),
		Key:    aws.String(dataObj.DataFileS3Key),
	}); err != nil {
		return nil, trace.NewAggregate(err, originalFile.Close())
	}

	defer originalFile.Close()

	// maxMemoryUsedForSortingExportInBytes defines how big chunks of audit events
	// will be loaded into memory, sorted and stored in temporary files.
	maxMemoryUsedForSortingExportInBytes := 1024 * 1024 * t.MaxMemoryUsedForSortingExportInMB
	sortedFile, err := createSortedExport(originalFile, dir, dataObj.ItemCount, maxMemoryUsedForSortingExportInBytes)
	return sortedFile, trace.Wrap(err)
}

type eventWithTime struct {
	rawLine   json.RawMessage
	eventDate string
}

// createSortedExport reads a large json.gz export file, which cannot fit entirely
// into memory and splits it into multiple smaller sorted (in memory) files.
// Afterwards, it merges these smaller files into a final sorted output.
// The function returns json file which is sorted by createdAtDate ascending.
func createSortedExport(inFile *os.File, dir string, itemCount, maxSize int) (f *os.File, err error) {
	tmpSortedFilesDir := path.Join(dir, fmt.Sprintf("sort-%s", uuid.NewString()))
	if err := os.Mkdir(tmpSortedFilesDir, 0o755); err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		err = trace.NewAggregate(err, os.RemoveAll(tmpSortedFilesDir))
	}()

	gzipReader, err := gzip.NewReader(inFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer gzipReader.Close()

	dec := json.NewDecoder(gzipReader)

	const averageEventSize = 500
	eventsCapacity := itemCount
	// If expected number of events * average size is grater then 1/4 of max size,
	// then set events capacity as 1/4 of max size / averageEventSize.
	if itemCount*averageEventSize > maxSize/4 {
		eventsCapacity = maxSize / averageEventSize / 4
	}

	events := make([]eventWithTime, 0, eventsCapacity)
	var tmpFiles []*os.File
	var size int
	// dec.More read export line by line because export is new line json.
	for dec.More() {
		// First decode whole line to rawMessage.
		var singleLine json.RawMessage
		err = dec.Decode(&singleLine)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Parse just createAtDate.
		var parsedLine dynamoEventPart
		err = json.Unmarshal(singleLine, &parsedLine)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		events = append(events, eventWithTime{
			rawLine:   singleLine,
			eventDate: parsedLine.Item.CreatedAtDate.Value,
		})
		size += len(singleLine) + len(parsedLine.Item.CreatedAtDate.Value)
		if size >= maxSize {
			// When max size is reached, sort events and write tmp file.
			tmpFile, err := sortEventsAndWriteTmpFile(tmpSortedFilesDir, events)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			tmpFiles = append(tmpFiles, tmpFile)
			// clear size and events slice, keeping the buffer.
			events = events[:0]
			size = 0
		}
	}
	// last batch, after last event was read from file.
	if size > 0 {
		tmpFile, err := sortEventsAndWriteTmpFile(tmpSortedFilesDir, events)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tmpFiles = append(tmpFiles, tmpFile)
	}
	finalFile, err := os.CreateTemp(dir, "final_sorted_*.json")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := mergeFiles(tmpFiles, finalFile); err != nil {
		return nil, trace.NewAggregate(err, finalFile.Close())
	}
	if _, err := finalFile.Seek(0, io.SeekStart); err != nil {
		return nil, trace.NewAggregate(err, finalFile.Close())
	}
	return finalFile, nil
}

func sortEventsAndWriteTmpFile(dir string, events []eventWithTime) (*os.File, error) {
	tmp, err := os.CreateTemp(dir, "tmp_sort_*.json")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].eventDate < events[j].eventDate
	})
	enc := json.NewEncoder(tmp)
	for _, eventWithTime := range events {
		err = enc.Encode(eventWithTime.rawLine)
		if err != nil {
			return nil, trace.NewAggregate(err, tmp.Close())
		}
	}
	// file reference will be used for reading later, so go to SeekStart.
	_, err = tmp.Seek(0, io.SeekStart)
	return tmp, trace.Wrap(err)
}

type stringVal struct {
	Value string `json:"S"`
}
type item struct {
	CreatedAtDate stringVal `json:"CreatedAtDate"`
}
type dynamoEventPart struct {
	Item item `json:"Item"`
}

// fileLine represents a line read from one of the input files.
// It contains the EventPart parsed from the JSON line, rawEvent
// and a decoder to read more lines objects from the file this line was read from.
type fileLine struct {
	// EventPart is part of event which contains only createdAtDate
	EventPart dynamoEventPart
	// RawEvent is full event line.
	RawEvent json.RawMessage
	// Dec is decoder which allows to read next items for given file.
	Dec *json.Decoder
}

// priorityQueue implements heap.Interface and holds FileLines.
// Mostly copied from https://cs.opensource.google/go/go/+/master:src/container/heap/example_pq_test.go
// without index field which is not needed when merging files.
type priorityQueue []*fileLine

func (pq *priorityQueue) Len() int { return len(*pq) }

func (pq *priorityQueue) Less(i, j int) bool {
	// We want a min heap, so use less for dates comparison.
	return (*pq)[i].EventPart.Item.CreatedAtDate.Value < (*pq)[j].EventPart.Item.CreatedAtDate.Value
}

func (pq *priorityQueue) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
}

func (pq *priorityQueue) Push(x interface{}) {
	item := x.(*fileLine)
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*pq = old[0 : n-1]
	return item
}

func readLine(dec *json.Decoder) (*fileLine, error) {
	var raw json.RawMessage
	// It will return EOF which is handled in caller.
	if err := dec.Decode(&raw); err != nil {
		return nil, trace.Wrap(err)
	}
	var dynamoEvent dynamoEventPart
	if err := json.Unmarshal(raw, &dynamoEvent); err != nil {
		return nil, trace.Wrap(err)
	}

	return &fileLine{
		EventPart: dynamoEvent,
		Dec:       dec,
		RawEvent:  raw,
	}, nil
}

// mergeFiles merges multiple sorted JSON files into a single output file.
// The function maintains the sorted order of JSON lines based on the
// 'CreatedAtDate' field.
func mergeFiles(files []*os.File, outputFile *os.File) error {
	finalFileEncoder := json.NewEncoder(outputFile)

	// A priority queue (heap) is used to efficiently select the smallest date from
	// the current line of each file.
	pq := &priorityQueue{}
	heap.Init(pq)

	// Open input files and read first line from each file to initialize
	// priority queue.
	for _, file := range files {
		dec := json.NewDecoder(file)
		line, err := readLine(dec)
		if err != nil {
			// on first line, there should be no error.
			return trace.Wrap(err)
		}
		heap.Push(pq, line)
	}

	// Consume first event from queue, write it to file and add next item from
	// that file to queue.
	for pq.Len() > 0 {
		min := heap.Pop(pq).(*fileLine)
		if err := finalFileEncoder.Encode(min.RawEvent); err != nil {
			return trace.Wrap(err)
		}

		nextLine, err := readLine(min.Dec)
		if trace.Unwrap(err) == io.EOF {
			// EOF means that file no longer has events. Continue with other files.
			continue
		} else if err != nil {
			return trace.Wrap(err)
		}
		heap.Push(pq, nextLine)
	}

	// Close all tmp files.
	var tmpCloseErrors []error
	for _, file := range files {
		tmpCloseErrors = append(tmpCloseErrors, file.Close())
	}
	return trace.NewAggregate(tmpCloseErrors...)
}

type checkpointData struct {
	ExportARN         string `json:"export_arn"`
	FinishedWithError bool   `json:"finished_with_error"`
	// Checkpoints key represents worker index.
	// Checkpoints value represents last valid event id.
	Checkpoints map[int]string `json:"checkpoints"`
}

func (c *checkpointData) checkpointValues() []string {
	if c == nil {
		return nil
	}
	return maps.Values(c.Checkpoints)
}

func (t *task) storeEmitterCheckpoint(in checkpointData) error {
	bb, err := json.Marshal(in)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.WriteFile(t.CheckpointPath, bb, 0o755))
}

func (t *task) loadEmitterCheckpoint(ctx context.Context, exportARN string) (*checkpointData, error) {
	bb, err := os.ReadFile(t.CheckpointPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}
	var out checkpointData
	if err := json.Unmarshal(bb, &out); err != nil {
		return nil, trace.Wrap(err)
	}

	// There are checkpoints for different export, assume there is no checkpoint saved.
	if exportARN != out.ExportARN {
		return nil, nil
	}

	return &out, nil
}

type eventWithErr struct {
	event apievents.AuditEvent
	err   error
}

func (t *task) emitEvents(ctx context.Context, eventsC <-chan apievents.AuditEvent, exportARN string) error {
	if t.DryRun {
		var invalidEvents []eventWithErr
		var count int
		var oldest, newest apievents.AuditEvent
		for event := range eventsC {
			count++
			if validateErr := validateEvent(event); validateErr != nil {
				invalidEvents = append(invalidEvents, eventWithErr{event: event, err: validateErr})
				continue
			}
			if oldest == nil && newest == nil {
				// first iteration, initialize values with first event.
				oldest = event
				newest = event
			}
			if oldest.GetTime().After(event.GetTime()) {
				oldest = event
			}
			if newest.GetTime().Before(event.GetTime()) {
				newest = event
			}
		}
		if count == 0 {
			return errors.New("there were not events from export")
		}
		if len(invalidEvents) > 0 {
			for _, eventWithErr := range invalidEvents {
				t.Logger.Debugf("Event %q %q %v is invalid: %v", eventWithErr.event.GetType(), eventWithErr.event.GetID(), eventWithErr.event.GetTime().Format(time.RFC3339), eventWithErr.err)
			}
			return trace.Errorf("there are %d invalid items", len(invalidEvents))
		}
		t.Logger.Infof("Dry run: there are %d events from %v to %v", count, oldest.GetTime(), newest.GetTime())
		return nil
	}
	// mu protects checkpointsPerWorker.
	var mu sync.Mutex
	checkpointsPerWorker := map[int]string{}

	errG, workerCtx := errgroup.WithContext(ctx)

	for i := 0; i < t.NoOfEmitWorkers; i++ {
		i := i
		errG.Go(func() error {
			for {
				select {
				case <-workerCtx.Done():
					return trace.Wrap(ctx.Err())
				case e, ok := <-eventsC:
					if !ok {
						return nil
					}
					if err := t.eventsEmitter.EmitAuditEvent(workerCtx, e); err != nil {
						return trace.Wrap(err)
					} else {
						mu.Lock()
						checkpointsPerWorker[i] = e.GetID()
						mu.Unlock()
					}
				}
			}
		})
	}

	workersErr := errG.Wait()
	// workersErr is handled below because we want to store checkpoint on error.

	// If there is missing data from at least one worker, it means that worker
	// does not have any valid checkpoint to store. Without any valid checkpoint
	// we won't be able to calculate min checkpoint, so does not store checkpoint at all.
	if len(checkpointsPerWorker) < t.NoOfEmitWorkers {
		t.Logger.Warnf("Not enough checkpoints from workers, got %d, expected %d", len(checkpointsPerWorker), t.NoOfEmitWorkers)
		return trace.Wrap(workersErr)
	}

	checkpoint := checkpointData{
		FinishedWithError: workersErr != nil || ctx.Err() != nil,
		ExportARN:         exportARN,
		Checkpoints:       checkpointsPerWorker,
	}
	if err := t.storeEmitterCheckpoint(checkpoint); err != nil {
		t.Logger.Errorf("Failed to store checkpoint: %v", err)
	}
	return trace.Wrap(workersErr)
}

func validateEvent(event apievents.AuditEvent) error {
	if event.GetTime().IsZero() {
		return trace.BadParameter("empty event time")
	}
	if _, err := uuid.Parse(event.GetID()); err != nil {
		return trace.BadParameter("invalid uid format: %v", err)
	}
	oneOf, err := apievents.ToOneOf(event)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = oneOf.Marshal()
	return trace.Wrap(err)
}
