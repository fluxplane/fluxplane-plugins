package aws

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type S3BucketsInput struct {
	RegionInput
	Prefix string `json:"prefix,omitempty" jsonschema:"description=Only buckets whose name starts with this prefix."`
}

type S3Bucket struct {
	Name    string `json:"name"`
	Created string `json:"created,omitempty"`
}

type S3BucketsResult struct {
	Region  string     `json:"region"`
	Buckets []S3Bucket `json:"buckets"`
	Count   int        `json:"count"`}

// S3Buckets lists the account's buckets.
func (s Service) S3Buckets(ctx pluginbinding.Context, input S3BucketsInput) (S3BucketsResult, error) {
	cfg, err := awsConfig(ctx, input.Region)
	if err != nil {
		return S3BucketsResult{}, err
	}
	callCtx, cancel := opContext()
	defer cancel()
	listed, err := s3.NewFromConfig(cfg).ListBuckets(callCtx, &s3.ListBucketsInput{})
	if err != nil {
		return S3BucketsResult{}, mapAWSError("s3 list-buckets", err)
	}
	prefix := strings.TrimSpace(input.Prefix)
	out := S3BucketsResult{Region: cfg.Region, Buckets: []S3Bucket{}}
	for _, bucket := range listed.Buckets {
		name := str(bucket.Name)
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		mapped := S3Bucket{Name: name}
		if bucket.CreationDate != nil {
			mapped.Created = bucket.CreationDate.UTC().Format(time.RFC3339)
		}
		out.Buckets = append(out.Buckets, mapped)
	}
	out.Count = len(out.Buckets)
	return out, nil
}

type S3ObjectsInput struct {
	RegionInput
	Bucket    string `json:"bucket,omitempty" jsonschema:"required,description=Bucket name."`
	Prefix    string `json:"prefix,omitempty" jsonschema:"description=Key prefix filter."`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum objects returned. Defaults to 100 and is capped at 1000.,minimum=0,maximum=1000"`
	NextToken string `json:"next_token,omitempty" jsonschema:"description=Continuation token from a previous truncated call."`
}

type S3Object struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	Modified     string `json:"modified,omitempty"`
	StorageClass string `json:"storage_class,omitempty"`
}

type S3ObjectsResult struct {
	Region    string     `json:"region"`
	Bucket    string     `json:"bucket"`
	Objects   []S3Object `json:"objects"`
	Count     int        `json:"count"`
	Truncated bool       `json:"truncated,omitempty"`
	NextToken string     `json:"next_token,omitempty"`}

// S3Objects lists objects under a prefix with pagination.
func (s Service) S3Objects(ctx pluginbinding.Context, input S3ObjectsInput) (S3ObjectsResult, error) {
	bucket := strings.TrimSpace(input.Bucket)
	if bucket == "" {
		return S3ObjectsResult{}, pluginbinding.Fail("bad_input", "bucket is required")
	}
	cfg, err := awsConfig(ctx, input.Region)
	if err != nil {
		return S3ObjectsResult{}, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	request := &s3.ListObjectsV2Input{Bucket: &bucket, MaxKeys: int32Ptr(int32(limit))}
	if prefix := strings.TrimSpace(input.Prefix); prefix != "" {
		request.Prefix = &prefix
	}
	if token := strings.TrimSpace(input.NextToken); token != "" {
		request.ContinuationToken = &token
	}
	callCtx, cancel := opContext()
	defer cancel()
	listed, err := s3.NewFromConfig(cfg).ListObjectsV2(callCtx, request)
	if err != nil {
		return S3ObjectsResult{}, mapAWSError("s3 list-objects-v2", err)
	}
	out := S3ObjectsResult{Region: cfg.Region, Bucket: bucket, Objects: []S3Object{}}
	for _, object := range listed.Contents {
		mapped := S3Object{Key: str(object.Key), StorageClass: string(object.StorageClass)}
		if object.Size != nil {
			mapped.Size = *object.Size
		}
		if object.LastModified != nil {
			mapped.Modified = object.LastModified.UTC().Format(time.RFC3339)
		}
		out.Objects = append(out.Objects, mapped)
	}
	out.Count = len(out.Objects)
	if listed.IsTruncated != nil && *listed.IsTruncated {
		out.Truncated = true
		out.NextToken = str(listed.NextContinuationToken)
	}
	return out, nil
}

func int32Ptr(value int32) *int32 { return &value }
