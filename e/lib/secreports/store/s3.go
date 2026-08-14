package store

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// S3Config is the S3 config.
type S3Config struct {
	// AWSConfig is the AWS config.
	AWSConfig aws.Config
	// S3URI is the S3 URI where the reports will be stored.
	S3URI string
}

func (c *S3Config) CheckAndSetDefaults() error {
	if c.S3URI == "" {
		return trace.BadParameter("missing parameter S3URI")
	}
	return nil
}

// NewS3 creates a new S3 store.
func NewS3(cfg S3Config) (*S3Store, error) {
	u, err := url.Parse(cfg.S3URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &S3Store{
		client:     s3.NewFromConfig(cfg.AWSConfig),
		backetName: u.Host,
		backetPath: strings.Trim(u.Path, "/"),
	}, nil

}

// S3Store is the AWS S3 store.
type S3Store struct {
	client     *s3.Client
	backetName string
	backetPath string
}

// SaveReportResult saves the report result.
func (s *S3Store) SaveReportResult(ctx context.Context, name string, result *pb.ReportResult) error {
	buff, err := json.Marshal(result)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.backetName),
		Key:    aws.String(filepath.Join(s.backetPath, name)),
		Body:   bytes.NewReader(buff),
	})
	if err != nil {
		return awsutils.ConvertS3Error(err)
	}
	return nil
}

// LoadReportResult loads the report result from the store.
func (s *S3Store) LoadReportResult(ctx context.Context, name string) (*pb.ReportResult, error) {
	obj, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.backetName),
		Key:    aws.String(filepath.Join(s.backetPath, name)),
	})
	if err != nil {
		return nil, awsutils.ConvertS3Error(err)
	}
	defer obj.Body.Close()
	body, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out pb.ReportResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}
