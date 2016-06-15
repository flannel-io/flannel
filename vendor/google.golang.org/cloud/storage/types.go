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

package storage

import (
	"encoding/base64"
	"io"
	"time"

	raw "google.golang.org/api/storage/v1"
)

// BucketAttrs represents the metadata for a Google Cloud Storage bucket.
type BucketAttrs struct {
	// Name is the name of the bucket.
	Name string

	// ACL is the list of access control rules on the bucket.
	ACL []ACLRule

	// DefaultObjectACL is the list of access controls to
	// apply to new objects when no object ACL is provided.
	DefaultObjectACL []ACLRule

	// Location is the location of the bucket. It defaults to "US".
	Location string

	// MetaGeneration is the metadata generation of the bucket.
	MetaGeneration int64

	// StorageClass is the storage class of the bucket. This defines
	// how objects in the bucket are stored and determines the SLA
	// and the cost of storage. Typical values are "STANDARD" and
	// "DURABLE_REDUCED_AVAILABILITY". Defaults to "STANDARD".
	StorageClass string

	// Created is the creation time of the bucket.
	Created time.Time
}

func newBucket(b *raw.Bucket) *BucketAttrs {
	if b == nil {
		return nil
	}
	bucket := &BucketAttrs{
		Name:           b.Name,
		Location:       b.Location,
		MetaGeneration: b.Metageneration,
		StorageClass:   b.StorageClass,
		Created:        convertTime(b.TimeCreated),
	}
	acl := make([]ACLRule, len(b.Acl))
	for i, rule := range b.Acl {
		acl[i] = ACLRule{
			Entity: ACLEntity(rule.Entity),
			Role:   ACLRole(rule.Role),
		}
	}
	bucket.ACL = acl
	objACL := make([]ACLRule, len(b.DefaultObjectAcl))
	for i, rule := range b.DefaultObjectAcl {
		objACL[i] = ACLRule{
			Entity: ACLEntity(rule.Entity),
			Role:   ACLRole(rule.Role),
		}
	}
	bucket.DefaultObjectACL = objACL
	return bucket
}

func toRawObjectACL(oldACL []ACLRule) []*raw.ObjectAccessControl {
	var acl []*raw.ObjectAccessControl
	if len(oldACL) > 0 {
		acl = make([]*raw.ObjectAccessControl, len(oldACL))
		for i, rule := range oldACL {
			acl[i] = &raw.ObjectAccessControl{
				Entity: string(rule.Entity),
				Role:   string(rule.Role),
			}
		}
	}
	return acl
}

// toRawBucket copies the editable attribute from b to the raw library's Bucket type.
func (b *BucketAttrs) toRawBucket() *raw.Bucket {
	var acl []*raw.BucketAccessControl
	if len(b.ACL) > 0 {
		acl = make([]*raw.BucketAccessControl, len(b.ACL))
		for i, rule := range b.ACL {
			acl[i] = &raw.BucketAccessControl{
				Entity: string(rule.Entity),
				Role:   string(rule.Role),
			}
		}
	}
	dACL := toRawObjectACL(b.DefaultObjectACL)
	return &raw.Bucket{
		Name:             b.Name,
		DefaultObjectAcl: dACL,
		Location:         b.Location,
		StorageClass:     b.StorageClass,
		Acl:              acl,
	}
}

// toRawObject copies the editable attributes from o to the raw library's Object type.
func (o ObjectAttrs) toRawObject(bucket string) *raw.Object {
	acl := toRawObjectACL(o.ACL)
	return &raw.Object{
		Bucket:             bucket,
		Name:               o.Name,
		ContentType:        o.ContentType,
		ContentEncoding:    o.ContentEncoding,
		ContentLanguage:    o.ContentLanguage,
		CacheControl:       o.CacheControl,
		ContentDisposition: o.ContentDisposition,
		Acl:                acl,
		Metadata:           o.Metadata,
	}
}

// ObjectAttrs represents the metadata for a Google Cloud Storage (GCS) object.
type ObjectAttrs struct {
	// Bucket is the name of the bucket containing this GCS object.
	// This field is read-only.
	Bucket string

	// Name is the name of the object within the bucket.
	// This field is read-only.
	Name string

	// ContentType is the MIME type of the object's content.
	ContentType string

	// ContentLanguage is the content language of the object's content.
	ContentLanguage string

	// CacheControl is the Cache-Control header to be sent in the response
	// headers when serving the object data.
	CacheControl string

	// ACL is the list of access control rules for the object.
	ACL []ACLRule

	// Owner is the owner of the object. This field is read-only.
	//
	// If non-zero, it is in the form of "user-<userId>".
	Owner string

	// Size is the length of the object's content. This field is read-only.
	Size int64

	// ContentEncoding is the encoding of the object's content.
	ContentEncoding string

	// ContentDisposition is the optional Content-Disposition header of the object
	// sent in the response headers.
	ContentDisposition string

	// MD5 is the MD5 hash of the object's content. This field is read-only.
	MD5 []byte

	// CRC32C is the CRC32 checksum of the object's content using
	// the Castagnoli93 polynomial. This field is read-only.
	CRC32C uint32

	// MediaLink is an URL to the object's content. This field is read-only.
	MediaLink string

	// Metadata represents user-provided metadata, in key/value pairs.
	// It can be nil if no metadata is provided.
	Metadata map[string]string

	// Generation is the generation number of the object's content.
	// This field is read-only.
	Generation int64

	// MetaGeneration is the version of the metadata for this
	// object at this generation. This field is used for preconditions
	// and for detecting changes in metadata. A metageneration number
	// is only meaningful in the context of a particular generation
	// of a particular object. This field is read-only.
	MetaGeneration int64

	// StorageClass is the storage class of the bucket.
	// This value defines how objects in the bucket are stored and
	// determines the SLA and the cost of storage. Typical values are
	// "STANDARD" and "DURABLE_REDUCED_AVAILABILITY".
	// It defaults to "STANDARD". This field is read-only.
	StorageClass string

	// Deleted is the time the object was deleted.
	// If not deleted, it is the zero value. This field is read-only.
	Deleted time.Time

	// Updated is the creation or modification time of the object.
	// For buckets with versioning enabled, changing an object's
	// metadata does not change this property. This field is read-only.
	Updated time.Time
}

// convertTime converts a time in RFC3339 format to time.Time.
// If any error occurs in parsing, the zero-value time.Time is silently returned.
func convertTime(t string) time.Time {
	var r time.Time
	if t != "" {
		r, _ = time.Parse(time.RFC3339, t)
	}
	return r
}

func newObject(o *raw.Object) *ObjectAttrs {
	if o == nil {
		return nil
	}
	acl := make([]ACLRule, len(o.Acl))
	for i, rule := range o.Acl {
		acl[i] = ACLRule{
			Entity: ACLEntity(rule.Entity),
			Role:   ACLRole(rule.Role),
		}
	}
	owner := ""
	if o.Owner != nil {
		owner = o.Owner.Entity
	}
	md5, _ := base64.StdEncoding.DecodeString(o.Md5Hash)
	var crc32c uint32
	d, err := base64.StdEncoding.DecodeString(o.Crc32c)
	if err == nil && len(d) == 4 {
		crc32c = uint32(d[0])<<24 + uint32(d[1])<<16 + uint32(d[2])<<8 + uint32(d[3])
	}
	return &ObjectAttrs{
		Bucket:          o.Bucket,
		Name:            o.Name,
		ContentType:     o.ContentType,
		ContentLanguage: o.ContentLanguage,
		CacheControl:    o.CacheControl,
		ACL:             acl,
		Owner:           owner,
		ContentEncoding: o.ContentEncoding,
		Size:            int64(o.Size),
		MD5:             md5,
		CRC32C:          crc32c,
		MediaLink:       o.MediaLink,
		Metadata:        o.Metadata,
		Generation:      o.Generation,
		MetaGeneration:  o.Metageneration,
		StorageClass:    o.StorageClass,
		Deleted:         convertTime(o.TimeDeleted),
		Updated:         convertTime(o.Updated),
	}
}

// Query represents a query to filter objects from a bucket.
type Query struct {
	// Delimiter returns results in a directory-like fashion.
	// Results will contain only objects whose names, aside from the
	// prefix, do not contain delimiter. Objects whose names,
	// aside from the prefix, contain delimiter will have their name,
	// truncated after the delimiter, returned in prefixes.
	// Duplicate prefixes are omitted.
	// Optional.
	Delimiter string

	// Prefix is the prefix filter to query objects
	// whose names begin with this prefix.
	// Optional.
	Prefix string

	// Versions indicates whether multiple versions of the same
	// object will be included in the results.
	Versions bool

	// Cursor is a previously-returned page token
	// representing part of the larger set of results to view.
	// Optional.
	Cursor string

	// MaxResults is the maximum number of items plus prefixes
	// to return. As duplicate prefixes are omitted,
	// fewer total results may be returned than requested.
	// The default page limit is used if it is negative or zero.
	MaxResults int
}

// ObjectList represents a list of objects returned from a bucket List call.
type ObjectList struct {
	// Results represent a list of object results.
	Results []*ObjectAttrs

	// Next is the continuation query to retrieve more
	// results with the same filtering criteria. If there
	// are no more results to retrieve, it is nil.
	Next *Query

	// Prefixes represents prefixes of objects
	// matching-but-not-listed up to and including
	// the requested delimiter.
	Prefixes []string
}

// contentTyper implements ContentTyper to enable an
// io.ReadCloser to specify its MIME type.
type contentTyper struct {
	io.Reader
	t string
}

func (c *contentTyper) ContentType() string {
	return c.t
}
