// Package s3 provides an implementation of the media.Store interface for AWS S3.
// It allows uploading, retrieving, and deleting files in an S3 bucket.
package s3

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/abhinavxd/libredesk/internal/media"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Opt holds configuration parameters specific to AWS S3.
type Opt struct {
	URL                string        `koanf:"url"`
	PublicURL          string        `koanf:"public_url"`
	AccessKey          string        `koanf:"aws_access_key_id"`
	SecretKey          string        `koanf:"aws_secret_access_key"`
	Region             string        `koanf:"aws_default_region"`
	Bucket             string        `koanf:"bucket"`
	BucketPath         string        `koanf:"bucket_path"`
	BucketType         string        `koanf:"bucket_type"`
	UploadURI          string        `koanf:"upload_uri"`
	Expiry             time.Duration `koanf:"expiry"`
	VirtualHostedStyle bool          `koanf:"virtual_hosted_style"`
}

// Client implements the media.Store interface using AWS S3.
type Client struct {
	s3      *awss3.Client
	presign *awss3.PresignClient
	opts    Opt
}

// New creates and initializes a new S3 client with the provided options.
func New(opt Opt) (media.Store, error) {
	if opt.Region == "" {
		opt.Region = "us-east-1"
	}
	opt.URL = strings.TrimRight(opt.URL, "/")
	opt.PublicURL = strings.TrimRight(opt.PublicURL, "/")

	// Default expiry duration for S3 URLs.
	if opt.Expiry.Seconds() < 1 {
		opt.Expiry = 7 * 24 * time.Hour
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(opt.Region),
	}
	if opt.AccessKey != "" || opt.SecretKey != "" {
		loadOpts = append(loadOpts,
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(opt.AccessKey, opt.SecretKey, "")),
		)
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("loading aws config: %w", err)
	}

	cl := awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		if opt.URL != "" {
			o.BaseEndpoint = aws.String(opt.URL)
			o.UsePathStyle = !opt.VirtualHostedStyle
		}
	})

	return &Client{
		s3:      cl,
		presign: awss3.NewPresignClient(cl),
		opts:    opt,
	}, nil
}

// Put uploads a file to S3 with the specified name, content type, and file content.
func (c *Client) Put(name string, cType string, file io.ReadSeeker) (string, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	in := &awss3.PutObjectInput{
		Bucket:      aws.String(c.opts.Bucket),
		Key:         aws.String(c.makeBucketPath(name)),
		Body:        file,
		ContentType: aws.String(cType),
	}
	if c.opts.BucketType == "public" {
		in.ACL = awss3types.ObjectCannedACLPublicRead
	}

	if _, err := c.s3.PutObject(context.Background(), in); err != nil {
		return "", fmt.Errorf("s3 put bucket=%q key=%q content_type=%q: %w", c.opts.Bucket, c.makeBucketPath(name), cType, err)
	}

	return name, nil
}

// GetURL generates a URL to access the file stored in S3.
func (c *Client) GetURL(name, disposition, fileName string) string {
	if c.opts.BucketType == "private" && c.opts.PublicURL == "" {
		out, err := c.presign.PresignGetObject(context.Background(), &awss3.GetObjectInput{
			Bucket:                     aws.String(c.opts.Bucket),
			Key:                        aws.String(c.makeBucketPath(name)),
			ResponseContentDisposition: aws.String(fmt.Sprintf("%s; filename=\"%s\"", disposition, fileName)),
		}, func(po *awss3.PresignOptions) {
			po.Expires = c.opts.Expiry
		})
		if err != nil {
			return ""
		}
		return out.URL
	}

	return c.makeFileURL(name)
}

// GetBlob retrieves the file content from S3 as a byte slice.
func (c *Client) GetBlob(uurl string) ([]byte, error) {
	if p, err := url.Parse(uurl); err != nil {
		uurl = filepath.Base(uurl)
	} else {
		uurl = filepath.Base(p.Path)
	}

	out, err := c.s3.GetObject(context.Background(), &awss3.GetObjectInput{
		Bucket: aws.String(c.opts.Bucket),
		Key:    aws.String(c.makeBucketPath(filepath.Base(uurl))),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	b, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Delete removes the file identified by name from S3.
func (c *Client) Delete(name string) error {
	_, err := c.s3.DeleteObject(context.Background(), &awss3.DeleteObjectInput{
		Bucket: aws.String(c.opts.Bucket),
		Key:    aws.String(c.makeBucketPath(name)),
	})
	return err
}

// makeBucketPath constructs the path for the file inside the bucket.
func (c *Client) makeBucketPath(name string) string {
	p := strings.TrimPrefix(strings.TrimSuffix(c.opts.BucketPath, "/"), "/")
	if p == "" {
		return name
	}
	return p + "/" + name
}

// makeFileURL constructs the file URL based on the configured endpoint style.
func (c *Client) makeFileURL(name string) string {
	key := c.makeBucketPath(name)
	if c.opts.PublicURL != "" {
		return joinBaseURL(c.opts.PublicURL, key)
	}
	if c.opts.URL != "" {
		if c.opts.VirtualHostedStyle {
			return buildVirtualHostedURL(c.opts.URL, c.opts.Bucket, key)
		}
		return buildPathStyleURL(c.opts.URL, c.opts.Bucket, key)
	}
	return buildAWSURL(c.opts.Region, c.opts.Bucket, key)
}

func buildAWSURL(region, bucket, key string) string {
	if region == "" || region == "us-east-1" {
		return "https://" + bucket + ".s3.amazonaws.com/" + encodeURLPath(key)
	}
	return "https://" + bucket + ".s3." + region + ".amazonaws.com/" + encodeURLPath(key)
}

func buildPathStyleURL(base, bucket, key string) string {
	return joinBaseURL(base, bucket, key)
}

func buildVirtualHostedURL(base, bucket, key string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return strings.TrimRight(base, "/") + "/" + encodeURLPath(key)
	}
	u.Host = bucket + "." + u.Host
	u.Path = joinURLPath(u.Path, key)
	return u.String()
}

func joinBaseURL(base string, parts ...string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		base = strings.TrimRight(base, "/")
		trimmed := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.Trim(p, "/")
			if p != "" {
				trimmed = append(trimmed, encodeURLPath(p))
			}
		}
		if len(trimmed) == 0 {
			return base
		}
		return base + "/" + strings.Join(trimmed, "/")
	}
	u.Path = joinURLPath(u.Path, parts...)
	return u.String()
}

func joinURLPath(basePath string, parts ...string) string {
	cleanParts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p != "" {
			cleanParts = append(cleanParts, encodeURLPath(p))
		}
	}
	basePath = strings.TrimRight(basePath, "/")
	if len(cleanParts) == 0 {
		if basePath == "" {
			return "/"
		}
		return basePath
	}
	if basePath == "" || basePath == "/" {
		return "/" + strings.Join(cleanParts, "/")
	}
	return basePath + "/" + strings.Join(cleanParts, "/")
}

func encodeURLPath(p string) string {
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

// Name returns the name of the storage implementation, which is "s3" in this case.
func (c *Client) Name() string {
	return "s3"
}

// SignedURLValidator returns nil as S3 handles its own presigned URL validation.
func (c *Client) SignedURLValidator() func(name, sig string, exp int64) bool {
	return nil
}
