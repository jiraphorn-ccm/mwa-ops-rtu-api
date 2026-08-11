// Package storage wraps AWS S3 for panel image uploads.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	ErrNotConfigured = errors.New("s3 not configured")
	ErrInvalidKey    = errors.New("invalid s3 key")
	ErrKeyOutOfScope = errors.New("s3 key out of app scope")
)

// Options is the S3 client configuration.
type Options struct {
	Region          string
	Bucket          string
	AppPrefix       string
	AccessKeyID     string
	SecretAccessKey string
	SignedURLTTL    time.Duration
}

// S3Client uploads objects and issues presigned GET URLs.
type S3Client struct {
	opts          Options
	client        *s3.Client
	presignClient *s3.PresignClient
}

// NewS3 builds an S3 client. Returns nil when the bucket is not configured.
func NewS3(ctx context.Context, opts Options) (*S3Client, error) {
	if strings.TrimSpace(opts.Bucket) == "" {
		return nil, nil
	}
	if opts.SignedURLTTL <= 0 {
		opts.SignedURLTTL = 24 * time.Hour
	}
	opts.AppPrefix = strings.Trim(opts.AppPrefix, "/")

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(opts.Region),
	}
	if opts.AccessKeyID != "" && opts.SecretAccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg)
	return &S3Client{
		opts:          opts,
		client:        s3Client,
		presignClient: s3.NewPresignClient(s3Client),
	}, nil
}

// Configured reports whether uploads are available.
func (c *S3Client) Configured() bool {
	return c != nil && c.opts.Bucket != ""
}

// Bucket returns the configured bucket name.
func (c *S3Client) Bucket() string {
	if c == nil {
		return ""
	}
	return c.opts.Bucket
}

func (c *S3Client) bucket() (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	return c.opts.Bucket, nil
}

func (c *S3Client) assertKeyInScope(key string) error {
	if key == "" || strings.Contains(key, "..") {
		return ErrInvalidKey
	}
	scope := c.opts.AppPrefix + "/"
	if !strings.HasPrefix(key, scope) {
		return ErrKeyOutOfScope
	}
	return nil
}

// Upload stores an object in the configured bucket.
func (c *S3Client) Upload(ctx context.Context, key, contentType string, body io.Reader) error {
	if err := c.assertKeyInScope(key); err != nil {
		return err
	}
	bucket, err := c.bucket()
	if err != nil {
		return err
	}
	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return err
}

// Delete removes an object from S3.
func (c *S3Client) Delete(ctx context.Context, key string) error {
	if err := c.assertKeyInScope(key); err != nil {
		return err
	}
	bucket, err := c.bucket()
	if err != nil {
		return err
	}
	_, err = c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

// PresignGet returns a signed GET URL for the object key.
func (c *S3Client) PresignGet(ctx context.Context, key string) (string, error) {
	if err := c.assertKeyInScope(key); err != nil {
		return "", err
	}
	bucket, err := c.bucket()
	if err != nil {
		return "", err
	}
	out, err := c.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(c.opts.SignedURLTTL))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

// PanelImageKey builds the S3 object key for a panel photo.
func PanelImageKey(appPrefix, panelCode, objectID, ext string) string {
	prefix := strings.Trim(appPrefix, "/")
	return fmt.Sprintf("%s/images/rtu/panels/%s/%s%s", prefix, panelCode, objectID, ext)
}

// AttachmentKey builds the S3 object key for a polymorphic attachment
// (rtu.attachments), grouped by the entity it is attached to.
func AttachmentKey(appPrefix, entityType, entityID, objectID, ext string) string {
	prefix := strings.Trim(appPrefix, "/")
	return fmt.Sprintf("%s/attachments/%s/%s/%s%s", prefix, strings.ToLower(entityType), entityID, objectID, ext)
}
