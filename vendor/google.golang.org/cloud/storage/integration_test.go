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
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/cloud"
	"google.golang.org/cloud/internal/testutil"
)

var (
	contents   = make(map[string][]byte)
	objects    = []string{"obj1", "obj2", "obj/with/slashes"}
	aclObjects = []string{"acl1", "acl2"}
	copyObj    = "copy-object"
)

const envBucket = "GCLOUD_TESTS_GOLANG_PROJECT_ID"

// testConfig returns the Client used to access GCS and the default bucket
// name to use.
func testConfig(ctx context.Context, t *testing.T) (*Client, string) {
	ts := cloud.WithTokenSource(testutil.TokenSource(ctx, ScopeFullControl))
	if ts == nil {
		t.Skip("Integration tests skipped. See CONTRIBUTING.md for details")
	}
	p := testutil.ProjID()
	if p == "" {
		log.Fatal("The project ID must be set. See CONTRIBUTING.md for details")
	}
	client, err := NewClient(ctx, ts)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client, p
}

func TestAdminClient(t *testing.T) {
	ctx := context.Background()
	projectID := testutil.ProjID()
	newBucket := projectID + "copy"

	client, err := NewAdminClient(ctx, projectID, cloud.WithTokenSource(testutil.TokenSource(ctx, ScopeFullControl)))
	if err != nil {
		t.Fatalf("Could not create client: %v", err)
	}
	defer client.Close()

	if err := client.CreateBucket(ctx, newBucket, nil); err != nil {
		t.Errorf("CreateBucket(%v, %v) failed %v", newBucket, nil, err)
	}
	if err := client.DeleteBucket(ctx, newBucket); err != nil {
		t.Errorf("DeleteBucket(%v) failed %v", newBucket, err)
		t.Logf("TODO: Warning this test left a new bucket in the cloud project, it must be deleted manually")
	}
	attrs := BucketAttrs{
		DefaultObjectACL: []ACLRule{{Entity: "domain-google.com", Role: RoleReader}},
	}
	if err := client.CreateBucket(ctx, newBucket, &attrs); err != nil {
		t.Errorf("CreateBucket(%v, %v) failed %v", newBucket, attrs, err)
	}
	if err := client.DeleteBucket(ctx, newBucket); err != nil {
		t.Errorf("DeleteBucket(%v) failed %v", newBucket, err)
		t.Logf("TODO: Warning this test left a new bucket in the cloud project, it must be deleted manually")
	}
}

func TestObjects(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}
	ctx := context.Background()
	client, bucket := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(bucket)

	// Cleanup.
	cleanup(t, "obj")

	const defaultType = "text/plain"

	// Test Writer.
	for _, obj := range objects {
		t.Logf("Writing %v", obj)
		wc := bkt.Object(obj).NewWriter(ctx)
		wc.ContentType = defaultType
		c := randomContents()
		if _, err := wc.Write(c); err != nil {
			t.Errorf("Write for %v failed with %v", obj, err)
		}
		if err := wc.Close(); err != nil {
			t.Errorf("Close for %v failed with %v", obj, err)
		}
		contents[obj] = c
	}

	// Test Reader.
	for _, obj := range objects {
		t.Logf("Creating a reader to read %v", obj)
		rc, err := bkt.Object(obj).NewReader(ctx)
		if err != nil {
			t.Errorf("Can't create a reader for %v, errored with %v", obj, err)
		}
		slurp, err := ioutil.ReadAll(rc)
		if err != nil {
			t.Errorf("Can't ReadAll object %v, errored with %v", obj, err)
		}
		if got, want := slurp, contents[obj]; !bytes.Equal(got, want) {
			t.Errorf("Contents (%q) = %q; want %q", obj, got, want)
		}
		if got, want := rc.Size(), len(contents[obj]); got != int64(want) {
			t.Errorf("Size (%q) = %d; want %d", obj, got, want)
		}
		if got, want := rc.ContentType(), "text/plain"; got != want {
			t.Errorf("ContentType (%q) = %q; want %q", obj, got, want)
		}
		rc.Close()

		// Test SignedURL
		opts := &SignedURLOptions{
			GoogleAccessID: "xxx@clientid",
			PrivateKey:     dummyKey("rsa"),
			Method:         "GET",
			MD5:            []byte("202cb962ac59075b964b07152d234b70"),
			Expires:        time.Date(2020, time.October, 2, 10, 0, 0, 0, time.UTC),
			ContentType:    "application/json",
			Headers:        []string{"x-header1", "x-header2"},
		}
		u, err := SignedURL(bucket, obj, opts)
		if err != nil {
			t.Fatalf("SignedURL(%q, %q) errored with %v", bucket, obj, err)
		}
		res, err := client.hc.Get(u)
		if err != nil {
			t.Fatalf("Can't get URL %q: %v", u, err)
		}
		slurp, err = ioutil.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("Can't ReadAll signed object %v, errored with %v", obj, err)
		}
		if got, want := slurp, contents[obj]; !bytes.Equal(got, want) {
			t.Errorf("Contents (%v) = %q; want %q", obj, got, want)
		}
		res.Body.Close()
	}

	// Test NotFound.
	_, err := bkt.Object("obj-not-exists").NewReader(ctx)
	if err != ErrObjectNotExist {
		t.Errorf("Object should not exist, err found to be %v", err)
	}

	name := objects[0]

	// Test StatObject.
	o, err := bkt.Object(name).Attrs(ctx)
	if err != nil {
		t.Error(err)
	}
	if got, want := o.Name, name; got != want {
		t.Errorf("Name (%v) = %q; want %q", name, got, want)
	}
	if got, want := o.ContentType, defaultType; got != want {
		t.Errorf("ContentType (%v) = %q; want %q", name, got, want)
	}

	// Test object copy.
	copy, err := client.CopyObject(ctx, bucket, name, bucket, copyObj, nil)
	if err != nil {
		t.Errorf("CopyObject failed with %v", err)
	}
	if copy.Name != copyObj {
		t.Errorf("Copy object's name = %q; want %q", copy.Name, copyObj)
	}
	if copy.Bucket != bucket {
		t.Errorf("Copy object's bucket = %q; want %q", copy.Bucket, bucket)
	}

	// Test UpdateAttrs.
	updated, err := bkt.Object(name).Update(ctx, ObjectAttrs{
		ContentType: "text/html",
		ACL:         []ACLRule{{Entity: "domain-google.com", Role: RoleReader}},
	})
	if err != nil {
		t.Errorf("UpdateAttrs failed with %v", err)
	}
	if want := "text/html"; updated.ContentType != want {
		t.Errorf("updated.ContentType == %q; want %q", updated.ContentType, want)
	}

	// Test checksums.
	checksumCases := []struct {
		name     string
		contents [][]byte
		size     int64
		md5      string
		crc32c   uint32
	}{
		{
			name:     "checksum-object",
			contents: [][]byte{[]byte("hello"), []byte("world")},
			size:     10,
			md5:      "fc5e038d38a57032085441e7fe7010b0",
			crc32c:   1456190592,
		},
		{
			name:     "zero-object",
			contents: [][]byte{},
			size:     0,
			md5:      "d41d8cd98f00b204e9800998ecf8427e",
			crc32c:   0,
		},
	}
	for _, c := range checksumCases {
		wc := bkt.Object(c.name).NewWriter(ctx)
		for _, data := range c.contents {
			if _, err := wc.Write(data); err != nil {
				t.Errorf("Write(%q) failed with %q", data, err)
			}
		}
		if err = wc.Close(); err != nil {
			t.Errorf("%q: close failed with %q", c.name, err)
		}
		obj := wc.Attrs()
		if got, want := obj.Size, c.size; got != want {
			t.Errorf("Object (%q) Size = %v; want %v", c.name, got, want)
		}
		if got, want := fmt.Sprintf("%x", obj.MD5), c.md5; got != want {
			t.Errorf("Object (%q) MD5 = %q; want %q", c.name, got, want)
		}
		if got, want := obj.CRC32C, c.crc32c; got != want {
			t.Errorf("Object (%q) CRC32C = %v; want %v", c.name, got, want)
		}
	}

	// Test public ACL.
	publicObj := objects[0]
	if err = bkt.Object(publicObj).ACL().Set(ctx, AllUsers, RoleReader); err != nil {
		t.Errorf("PutACLEntry failed with %v", err)
	}
	publicClient, err := NewClient(ctx, cloud.WithBaseHTTP(http.DefaultClient))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := publicClient.Bucket(bucket).Object(publicObj).NewReader(ctx)
	if err != nil {
		t.Error(err)
	}
	slurp, err := ioutil.ReadAll(rc)
	if err != nil {
		t.Errorf("ReadAll failed with %v", err)
	}
	if !bytes.Equal(slurp, contents[publicObj]) {
		t.Errorf("Public object's content: got %q, want %q", slurp, contents[publicObj])
	}
	rc.Close()

	// Test writer error handling.
	wc := publicClient.Bucket(bucket).Object(publicObj).NewWriter(ctx)
	if _, err := wc.Write([]byte("hello")); err != nil {
		t.Errorf("Write unexpectedly failed with %v", err)
	}
	if err = wc.Close(); err == nil {
		t.Error("Close expected an error, found none")
	}

	// DeleteObject object.
	// The rest of the other object will be deleted during
	// the initial cleanup. This tests exists, so we still can cover
	// deletion if there are no objects on the bucket to clean.
	if err := bkt.Object(copyObj).Delete(ctx); err != nil {
		t.Errorf("Deletion of %v failed with %v", copyObj, err)
	}
	_, err = bkt.Object(copyObj).Attrs(ctx)
	if err != ErrObjectNotExist {
		t.Errorf("Copy is expected to be deleted, stat errored with %v", err)
	}
}

func TestACL(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}
	ctx := context.Background()
	client, bucket := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(bucket)

	cleanup(t, "acl")
	entity := ACLEntity("domain-google.com")
	if err := client.Bucket(bucket).DefaultObjectACL().Set(ctx, entity, RoleReader); err != nil {
		t.Errorf("Can't put default ACL rule for the bucket, errored with %v", err)
	}
	for _, obj := range aclObjects {
		t.Logf("Writing %v", obj)
		wc := bkt.Object(obj).NewWriter(ctx)
		c := randomContents()
		if _, err := wc.Write(c); err != nil {
			t.Errorf("Write for %v failed with %v", obj, err)
		}
		if err := wc.Close(); err != nil {
			t.Errorf("Close for %v failed with %v", obj, err)
		}
	}
	name := aclObjects[0]
	o := bkt.Object(name)
	acl, err := o.ACL().List(ctx)
	if err != nil {
		t.Errorf("Can't retrieve ACL of %v", name)
	}
	aclFound := false
	for _, rule := range acl {
		if rule.Entity == entity && rule.Role == RoleReader {
			aclFound = true
		}
	}
	if !aclFound {
		t.Error("Expected to find an ACL rule for google.com domain users, but not found")
	}
	if err := o.ACL().Delete(ctx, entity); err != nil {
		t.Errorf("Can't delete the ACL rule for the entity: %v", entity)
	}

	if err := bkt.ACL().Set(ctx, "user-jbd@google.com", RoleReader); err != nil {
		t.Errorf("Error while putting bucket ACL rule: %v", err)
	}
	bACL, err := bkt.ACL().List(ctx)
	if err != nil {
		t.Errorf("Error while getting the ACL of the bucket: %v", err)
	}
	bACLFound := false
	for _, rule := range bACL {
		if rule.Entity == "user-jbd@google.com" && rule.Role == RoleReader {
			bACLFound = true
		}
	}
	if !bACLFound {
		t.Error("Expected to find an ACL rule for jbd@google.com user, but not found")
	}
	if err := bkt.ACL().Delete(ctx, "user-jbd@google.com"); err != nil {
		t.Errorf("Error while deleting bucket ACL rule: %v", err)
	}
}

func TestValidObjectNames(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}
	ctx := context.Background()
	client, bucket := testConfig(ctx, t)
	defer client.Close()

	bkt := client.Bucket(bucket)

	validNames := []string{
		"gopher",
		"Гоферови",
		"a",
		strings.Repeat("a", 1024),
	}
	for _, name := range validNames {
		w := bkt.Object(name).NewWriter(ctx)
		if _, err := w.Write([]byte("data")); err != nil {
			t.Errorf("Object %q write failed: %v. Want success", name, err)
			continue
		}
		if err := w.Close(); err != nil {
			t.Errorf("Object %q close failed: %v. Want success", name, err)
			continue
		}
		defer bkt.Object(name).Delete(ctx)
	}

	invalidNames := []string{
		"", // Too short.
		strings.Repeat("a", 1025), // Too long.
		"new\nlines",
		"bad\xffunicode",
	}
	for _, name := range invalidNames {
		w := bkt.Object(name).NewWriter(ctx)
		// Invalid object names will either cause failure during Write or Close.
		if _, err := w.Write([]byte("data")); err != nil {
			continue
		}
		if err := w.Close(); err != nil {
			continue
		}
		defer bkt.Object(name).Delete(ctx)
		t.Errorf("%q should have failed. Didn't", name)
	}
}

func cleanup(t *testing.T, prefix string) {
	if testing.Short() {
		t.Skip("Integration tests cleanup skipped in short mode")
	}
	ctx := context.Background()
	client, bucket := testConfig(ctx, t)
	defer client.Close()

	var q *Query = &Query{
		Prefix: prefix,
	}
	for {
		o, err := client.Bucket(bucket).List(ctx, q)
		if err != nil {
			t.Fatalf("Cleanup List for bucket %v failed with error: %v", bucket, err)
		}
		for _, obj := range o.Results {
			t.Logf("Cleanup deletion of %v", obj.Name)
			if err = client.Bucket(bucket).Object(obj.Name).Delete(ctx); err != nil {
				t.Fatalf("Cleanup Delete for object %v failed with %v", obj.Name, err)
			}
		}
		if o.Next == nil {
			break
		}
		q = o.Next
	}
}

func randomContents() []byte {
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("hello world%d", rand.Intn(100000)))
	return h.Sum(nil)
}
