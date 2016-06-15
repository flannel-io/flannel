// Copyright 2014 Google Inc. All Rights Reserved.
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

// Package storage contains a Google Cloud Storage client.
//
// This package is experimental and may make backwards-incompatible changes.
package storage // import "google.golang.org/cloud/storage"

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/cloud"
	"google.golang.org/cloud/internal/transport"

	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"
	raw "google.golang.org/api/storage/v1"
)

var (
	ErrBucketNotExist = errors.New("storage: bucket doesn't exist")
	ErrObjectNotExist = errors.New("storage: object doesn't exist")
)

const userAgent = "gcloud-golang-storage/20151204"

const (
	// ScopeFullControl grants permissions to manage your
	// data and permissions in Google Cloud Storage.
	ScopeFullControl = raw.DevstorageFullControlScope

	// ScopeReadOnly grants permissions to
	// view your data in Google Cloud Storage.
	ScopeReadOnly = raw.DevstorageReadOnlyScope

	// ScopeReadWrite grants permissions to manage your
	// data in Google Cloud Storage.
	ScopeReadWrite = raw.DevstorageReadWriteScope
)

// AdminClient is a client type for performing admin operations on a project's
// buckets.
type AdminClient struct {
	hc        *http.Client
	raw       *raw.Service
	projectID string
}

// NewAdminClient creates a new AdminClient for a given project.
func NewAdminClient(ctx context.Context, projectID string, opts ...cloud.ClientOption) (*AdminClient, error) {
	c, err := NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &AdminClient{
		hc:        c.hc,
		raw:       c.raw,
		projectID: projectID,
	}, nil
}

// Close closes the AdminClient.
func (c *AdminClient) Close() error {
	c.hc = nil
	return nil
}

// Create creates a Bucket in the project.
// If attrs is nil the API defaults will be used.
func (c *AdminClient) CreateBucket(ctx context.Context, bucketName string, attrs *BucketAttrs) error {
	var bkt *raw.Bucket
	if attrs != nil {
		bkt = attrs.toRawBucket()
	} else {
		bkt = &raw.Bucket{}
	}
	bkt.Name = bucketName
	req := c.raw.Buckets.Insert(c.projectID, bkt)
	_, err := req.Context(ctx).Do()
	return err
}

// Delete deletes a Bucket in the project.
func (c *AdminClient) DeleteBucket(ctx context.Context, bucketName string) error {
	req := c.raw.Buckets.Delete(bucketName)
	return req.Context(ctx).Do()
}

// Client is a client for interacting with Google Cloud Storage.
type Client struct {
	hc  *http.Client
	raw *raw.Service
}

// NewClient creates a new Google Cloud Storage client.
// The default scope is ScopeFullControl. To use a different scope, like ScopeReadOnly, use cloud.WithScopes.
func NewClient(ctx context.Context, opts ...cloud.ClientOption) (*Client, error) {
	o := []cloud.ClientOption{
		cloud.WithScopes(ScopeFullControl),
		cloud.WithUserAgent(userAgent),
	}
	opts = append(o, opts...)
	hc, _, err := transport.NewHTTPClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("dialing: %v", err)
	}
	rawService, err := raw.New(hc)
	if err != nil {
		return nil, fmt.Errorf("storage client: %v", err)
	}
	return &Client{
		hc:  hc,
		raw: rawService,
	}, nil
}

// Close closes the Client.
func (c *Client) Close() error {
	c.hc = nil
	return nil
}

// BucketHandle provides operations on a Google Cloud Storage bucket.
// Use Client.Bucket to get a handle.
type BucketHandle struct {
	acl              *ACLHandle
	defaultObjectACL *ACLHandle

	c    *Client
	name string
}

// Bucket returns a BucketHandle, which provides operations on the named bucket.
// This call does not perform any network operations.
//
// name must contain only lowercase letters, numbers, dashes, underscores, and
// dots. The full specification for valid bucket names can be found at:
//   https://cloud.google.com/storage/docs/bucket-naming
func (c *Client) Bucket(name string) *BucketHandle {
	return &BucketHandle{
		c:    c,
		name: name,
		acl: &ACLHandle{
			c:      c,
			bucket: name,
		},
		defaultObjectACL: &ACLHandle{
			c:         c,
			bucket:    name,
			isDefault: true,
		},
	}
}

// ACL returns an ACLHandle, which provides access to the bucket's access control list.
// This controls who can list, create or overwrite the objects in a bucket.
// This call does not perform any network operations.
func (c *BucketHandle) ACL() *ACLHandle {
	return c.acl
}

// DefaultObjectACL returns an ACLHandle, which provides access to the bucket's default object ACLs.
// These ACLs are applied to newly created objects in this bucket that do not have a defined ACL.
// This call does not perform any network operations.
func (c *BucketHandle) DefaultObjectACL() *ACLHandle {
	return c.defaultObjectACL
}

// ObjectHandle provides operations on an object in a Google Cloud Storage bucket.
// Use BucketHandle.Object to get a handle.
type ObjectHandle struct {
	c      *Client
	bucket string
	object string

	acl *ACLHandle
}

// Object returns an ObjectHandle, which provides operations on the named object.
// This call does not perform any network operations.
//
// name must consist entirely of valid UTF-8-encoded runes. The full specification
// for valid object names can be found at:
//   https://cloud.google.com/storage/docs/bucket-naming
func (b *BucketHandle) Object(name string) *ObjectHandle {
	return &ObjectHandle{
		c:      b.c,
		bucket: b.name,
		object: name,
		acl: &ACLHandle{
			c:      b.c,
			bucket: b.name,
			object: name,
		},
	}
}

// TODO(jbd): Add storage.buckets.list.
// TODO(jbd): Add storage.buckets.update.

// TODO(jbd): Add storage.objects.watch.

// Attrs returns the metadata for the bucket.
func (b *BucketHandle) Attrs(ctx context.Context) (*BucketAttrs, error) {
	resp, err := b.c.raw.Buckets.Get(b.name).Projection("full").Context(ctx).Do()
	if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusNotFound {
		return nil, ErrBucketNotExist
	}
	if err != nil {
		return nil, err
	}
	return newBucket(resp), nil
}

// List lists objects from the bucket. You can specify a query
// to filter the results. If q is nil, no filtering is applied.
func (b *BucketHandle) List(ctx context.Context, q *Query) (*ObjectList, error) {
	req := b.c.raw.Objects.List(b.name)
	req.Projection("full")
	if q != nil {
		req.Delimiter(q.Delimiter)
		req.Prefix(q.Prefix)
		req.Versions(q.Versions)
		req.PageToken(q.Cursor)
		if q.MaxResults > 0 {
			req.MaxResults(int64(q.MaxResults))
		}
	}
	resp, err := req.Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	objects := &ObjectList{
		Results:  make([]*ObjectAttrs, len(resp.Items)),
		Prefixes: make([]string, len(resp.Prefixes)),
	}
	for i, item := range resp.Items {
		objects.Results[i] = newObject(item)
	}
	for i, prefix := range resp.Prefixes {
		objects.Prefixes[i] = prefix
	}
	if resp.NextPageToken != "" {
		next := Query{}
		if q != nil {
			// keep the other filtering
			// criteria if there is a query
			next = *q
		}
		next.Cursor = resp.NextPageToken
		objects.Next = &next
	}
	return objects, nil
}

// SignedURLOptions allows you to restrict the access to the signed URL.
type SignedURLOptions struct {
	// GoogleAccessID represents the authorizer of the signed URL generation.
	// It is typically the Google service account client email address from
	// the Google Developers Console in the form of "xxx@developer.gserviceaccount.com".
	// Required.
	GoogleAccessID string

	// PrivateKey is the Google service account private key. It is obtainable
	// from the Google Developers Console.
	// At https://console.developers.google.com/project/<your-project-id>/apiui/credential,
	// create a service account client ID or reuse one of your existing service account
	// credentials. Click on the "Generate new P12 key" to generate and download
	// a new private key. Once you download the P12 file, use the following command
	// to convert it into a PEM file.
	//
	//    $ openssl pkcs12 -in key.p12 -passin pass:notasecret -out key.pem -nodes
	//
	// Provide the contents of the PEM file as a byte slice.
	// Required.
	PrivateKey []byte

	// Method is the HTTP method to be used with the signed URL.
	// Signed URLs can be used with GET, HEAD, PUT, and DELETE requests.
	// Required.
	Method string

	// Expires is the expiration time on the signed URL. It must be
	// a datetime in the future.
	// Required.
	Expires time.Time

	// ContentType is the content type header the client must provide
	// to use the generated signed URL.
	// Optional.
	ContentType string

	// Headers is a list of extention headers the client must provide
	// in order to use the generated signed URL.
	// Optional.
	Headers []string

	// MD5 is the base64 encoded MD5 checksum of the file.
	// If provided, the client should provide the exact value on the request
	// header in order to use the signed URL.
	// Optional.
	MD5 []byte
}

// SignedURL returns a URL for the specified object. Signed URLs allow
// the users access to a restricted resource for a limited time without having a
// Google account or signing in. For more information about the signed
// URLs, see https://cloud.google.com/storage/docs/accesscontrol#Signed-URLs.
func SignedURL(bucket, name string, opts *SignedURLOptions) (string, error) {
	if opts == nil {
		return "", errors.New("storage: missing required SignedURLOptions")
	}
	if opts.GoogleAccessID == "" || opts.PrivateKey == nil {
		return "", errors.New("storage: missing required credentials to generate a signed URL")
	}
	if opts.Method == "" {
		return "", errors.New("storage: missing required method option")
	}
	if opts.Expires.IsZero() {
		return "", errors.New("storage: missing required expires option")
	}
	key, err := parseKey(opts.PrivateKey)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", opts.Method)
	fmt.Fprintf(h, "%s\n", opts.MD5)
	fmt.Fprintf(h, "%s\n", opts.ContentType)
	fmt.Fprintf(h, "%d\n", opts.Expires.Unix())
	fmt.Fprintf(h, "%s", strings.Join(opts.Headers, "\n"))
	fmt.Fprintf(h, "/%s/%s", bucket, name)
	b, err := rsa.SignPKCS1v15(
		rand.Reader,
		key,
		crypto.SHA256,
		h.Sum(nil),
	)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(b)
	u := &url.URL{
		Scheme: "https",
		Host:   "storage.googleapis.com",
		Path:   fmt.Sprintf("/%s/%s", bucket, name),
	}
	q := u.Query()
	q.Set("GoogleAccessId", opts.GoogleAccessID)
	q.Set("Expires", fmt.Sprintf("%d", opts.Expires.Unix()))
	q.Set("Signature", string(encoded))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ACL provides access to the object's access control list.
// This controls who can read and write this object.
// This call does not perform any network operations.
func (o *ObjectHandle) ACL() *ACLHandle {
	return o.acl
}

// Attrs returns meta information about the object.
// ErrObjectNotExist will be returned if the object is not found.
func (o *ObjectHandle) Attrs(ctx context.Context) (*ObjectAttrs, error) {
	if !utf8.ValidString(o.object) {
		return nil, fmt.Errorf("storage: object name %q is not valid UTF-8", o.object)
	}
	obj, err := o.c.raw.Objects.Get(o.bucket, o.object).Projection("full").Context(ctx).Do()
	if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusNotFound {
		return nil, ErrObjectNotExist
	}
	if err != nil {
		return nil, err
	}
	return newObject(obj), nil
}

// Update updates an object with the provided attributes.
// All zero-value attributes are ignored.
// ErrObjectNotExist will be returned if the object is not found.
func (o *ObjectHandle) Update(ctx context.Context, attrs ObjectAttrs) (*ObjectAttrs, error) {
	if !utf8.ValidString(o.object) {
		return nil, fmt.Errorf("storage: object name %q is not valid UTF-8", o.object)
	}
	obj, err := o.c.raw.Objects.Patch(o.bucket, o.object, attrs.toRawObject(o.bucket)).Projection("full").Context(ctx).Do()
	if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusNotFound {
		return nil, ErrObjectNotExist
	}
	if err != nil {
		return nil, err
	}
	return newObject(obj), nil
}

// Delete deletes the single specified object.
func (o *ObjectHandle) Delete(ctx context.Context) error {
	if !utf8.ValidString(o.object) {
		return fmt.Errorf("storage: object name %q is not valid UTF-8", o.object)
	}
	return o.c.raw.Objects.Delete(o.bucket, o.object).Context(ctx).Do()
}

// CopyObject copies the source object to the destination.
// The copied object's attributes are overwritten by attrs if non-nil.
func (c *Client) CopyObject(ctx context.Context, srcBucket, srcName string, destBucket, destName string, attrs *ObjectAttrs) (*ObjectAttrs, error) {
	if srcBucket == "" || destBucket == "" {
		return nil, errors.New("storage: srcBucket and destBucket must both be non-empty")
	}
	if srcName == "" || destName == "" {
		return nil, errors.New("storage: srcName and destName must be non-empty")
	}
	if !utf8.ValidString(srcName) {
		return nil, fmt.Errorf("storage: srcName %q is not valid UTF-8", srcName)
	}
	if !utf8.ValidString(destName) {
		return nil, fmt.Errorf("storage: destName %q is not valid UTF-8", destName)
	}
	var rawObject *raw.Object
	if attrs != nil {
		attrs.Name = destName
		if attrs.ContentType == "" {
			return nil, errors.New("storage: attrs.ContentType must be non-empty")
		}
		rawObject = attrs.toRawObject(destBucket)
	}
	o, err := c.raw.Objects.Copy(
		srcBucket, srcName, destBucket, destName, rawObject).Projection("full").Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return newObject(o), nil
}

// NewReader creates a new Reader to read the contents of the
// object.
// ErrObjectNotExist will be returned if the object is not found.
func (o *ObjectHandle) NewReader(ctx context.Context) (*Reader, error) {
	if !utf8.ValidString(o.object) {
		return nil, fmt.Errorf("storage: object name %q is not valid UTF-8", o.object)
	}
	u := &url.URL{
		Scheme: "https",
		Host:   "storage.googleapis.com",
		Path:   fmt.Sprintf("/%s/%s", o.bucket, o.object),
	}
	res, err := o.c.hc.Get(u.String())
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		res.Body.Close()
		return nil, ErrObjectNotExist
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		res.Body.Close()
		return nil, fmt.Errorf("storage: can't read object %v/%v, status code: %v", o.bucket, o.object, res.Status)
	}
	return &Reader{
		body:        res.Body,
		size:        res.ContentLength,
		contentType: res.Header.Get("Content-Type"),
	}, nil
}

// NewWriter returns a storage Writer that writes to the GCS object
// associated with this ObjectHandle.
// If such an object doesn't exist, it creates one.
// Attributes can be set on the object by modifying the returned Writer's
// ObjectAttrs field before the first call to Write.
//
// It is the caller's responsibility to call Close when writing is done.
//
// The object is not available and any previous object with the same
// name is not replaced on Cloud Storage until Close is called.
func (o *ObjectHandle) NewWriter(ctx context.Context) *Writer {
	return &Writer{
		ctx:         ctx,
		client:      o.c,
		bucket:      o.bucket,
		name:        o.object,
		donec:       make(chan struct{}),
		ObjectAttrs: ObjectAttrs{Name: o.object},
	}
}

// parseKey converts the binary contents of a private key file
// to an *rsa.PrivateKey. It detects whether the private key is in a
// PEM container or not. If so, it extracts the the private key
// from PEM container before conversion. It only supports PEM
// containers with no passphrase.
func parseKey(key []byte) (*rsa.PrivateKey, error) {
	if block, _ := pem.Decode(key); block != nil {
		key = block.Bytes
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(key)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(key)
		if err != nil {
			return nil, err
		}
	}
	parsed, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("oauth2: private key is invalid")
	}
	return parsed, nil
}
