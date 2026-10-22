/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gravitational/trace"
)

type objectStore interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type handoffTool struct {
	store  objectStore
	env    environmentConfig
	caller string
	now    func() time.Time
}

func selectCAB(keys []string) (string, error) {
	var candidates []string
	for _, key := range keys {
		if strings.HasSuffix(key, ".cab") {
			candidates = append(candidates, key)
		}
	}
	if len(candidates) == 0 {
		return "", trace.NotFound("no CAB found; choose a version that has already been generated")
	}
	if len(candidates) > 1 {
		return "", trace.BadParameter("expected one CAB for this version, found %d; generate a CAB with a new unique version", len(candidates))
	}
	return candidates[0], nil
}

func (tool *handoffTool) download(ctx context.Context, rawVersion, directory string) error {
	version, err := normalizeVersion(rawVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	if entries, err := os.ReadDir(directory); err == nil {
		if len(entries) != 0 {
			return trace.BadParameter("output directory must be empty")
		}
	} else if !os.IsNotExist(err) {
		return trace.Wrap(err)
	}

	paginator := s3.NewListObjectsV2Paginator(tool.store, &s3.ListObjectsV2Input{
		Bucket: aws.String(tool.env.UnsignedBucket), Prefix: aws.String(path.Join(tool.env.Name, version) + "/"),
	})
	var keys []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return trace.Wrap(err, "list CAB artifacts")
		}
		for _, object := range page.Contents {
			keys = append(keys, aws.ToString(object.Key))
		}
	}
	cabKey, err := selectCAB(keys)
	if err != nil {
		return trace.Wrap(err, "select CAB under s3://%s/%s/", tool.env.UnsignedBucket, path.Join(tool.env.Name, version))
	}

	if err := os.MkdirAll(directory, 0700); err != nil {
		return trace.Wrap(err)
	}
	for _, key := range keys {
		if key != cabKey && !strings.HasPrefix(key, cabKey+".") {
			continue
		}
		// Download only objects alongside the CAB, preserving their filenames.
		if path.Dir(key) != path.Dir(cabKey) {
			continue
		}
		if err := tool.downloadObject(ctx, key, filepath.Join(directory, path.Base(key))); err != nil {
			return trace.Wrap(err, "download incomplete; retry with an empty directory")
		}
	}

	receipt := downloadReceipt{CABS3Key: cabKey, CABDownloadedAt: tool.now().UTC(), CABDownloadedByAWSARN: tool.caller}
	if _, err := writeJSON(filepath.Join(directory, "download.json"), receipt); err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintf(os.Stderr, "CAB to submit: %s\n", filepath.Join(directory, path.Base(cabKey)))
	return nil
}

func (tool *handoffTool) downloadObject(ctx context.Context, key, filename string) error {
	object, err := tool.store.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(tool.env.UnsignedBucket), Key: aws.String(key)})
	if err != nil {
		return trace.Wrap(err, "download %s", key)
	}
	defer object.Body.Close()

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return trace.Wrap(err)
	}
	_, copyErr := io.Copy(file, object.Body)

	return trace.NewAggregate(copyErr, file.Close())
}

func (tool *handoffTool) upload(ctx context.Context, directory, zipPath, note string) error {
	var receipt downloadReceipt
	if err := readJSON(filepath.Join(directory, "download.json"), &receipt); err != nil {
		return trace.Wrap(err)
	}
	var metadata prepareMetadata
	if err := readJSON(filepath.Join(directory, path.Base(receipt.CABS3Key)+".metadata.json"), &metadata); err != nil {
		return trace.Wrap(err)
	}

	version, err := normalizeVersion(metadata.WindowsauthVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	prefix := path.Join(tool.env.Name, version)
	if !strings.HasPrefix(metadata.CAB.S3Key, prefix+"/") {
		return trace.BadParameter("CAB destination must match the selected environment and version")
	}

	filename := filepath.Base(zipPath)
	match := zipNamePattern.FindStringSubmatch(filename)
	if match == nil {
		return trace.BadParameter("ZIP filename must be Signed_<id>.zip")
	}

	digest, err := fileChecksum(zipPath)
	if err != nil {
		return trace.Wrap(err)
	}

	file, err := os.Open(zipPath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()

	zipKey := path.Join(prefix, filename)
	result, err := tool.store.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(tool.env.SignedBucket), Key: aws.String(zipKey), Body: file,
		ChecksumAlgorithm: types.ChecksumAlgorithmCrc32,
	})
	if err != nil {
		return trace.Wrap(err, "upload returned ZIP")
	}

	versionID := aws.ToString(result.VersionId)
	if versionID == "" || versionID == "null" {
		return trace.BadParameter("ZIP uploaded but no submission published: the signed bucket must have versioning enabled")
	}

	submission := submissionRecord{
		SchemaVersion:      1,
		WindowsauthVersion: metadata.WindowsauthVersion,
		PrepareCABS3Key:    metadata.CAB.S3Key,
		PrepareCABSHA256:   metadata.SignedCABSHA256,
		MicrosoftSubmission: microsoftSubmission{
			ID:                     match[1],
			ReturnedZIPFilename:    filename,
			ReturnedZIPSHA256:      digest,
			ReturnedZIPS3Key:       zipKey,
			ReturnedZIPS3VersionID: versionID,
		},
		UploadedByAWSARN:      tool.caller,
		CABDownloadedByAWSARN: receipt.CABDownloadedByAWSARN,
		CABDownloadedAt:       receipt.CABDownloadedAt,
		UploadedAt:            tool.now().UTC(),
		OperatorNote:          note,
	}
	submissionPath := filepath.Join(directory, "submission.json")
	data, err := writeJSON(submissionPath, submission)
	if err != nil {
		return trace.Wrap(err)
	}

	// Publish the companion record last, pinned to the exact uploaded ZIP version.
	_, err = tool.store.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(tool.env.SignedBucket), Key: aws.String(path.Join(prefix, "submission.json")),
		Body: bytes.NewReader(data), ContentType: aws.String("application/json"),
		ChecksumAlgorithm: types.ChecksumAlgorithmCrc32,
	})

	if err != nil {
		return trace.Wrap(err, "upload submission.json; retry the same command")
	}
	fmt.Fprintf(os.Stderr, "Uploaded ZIP and submission.json to s3://%s/%s/\n", tool.env.SignedBucket, prefix)
	return nil
}
