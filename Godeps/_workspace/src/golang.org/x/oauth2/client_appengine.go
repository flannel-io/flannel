// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build appengine

// App Engine hooks.

package oauth2

import (
	"net/http"

	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/flannel/Godeps/_workspace/src/golang.org/x/oauth2/internal"
	"github.com/coreos/flannel/Godeps/_workspace/src/google.golang.org/appengine/urlfetch"
)

func init() {
	internal.RegisterContextClientFunc(contextClientAppEngine)
}

func contextClientAppEngine(ctx context.Context) (*http.Client, error) {
	return urlfetch.Client(ctx), nil
}
