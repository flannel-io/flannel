// Copyright 2018 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import "testing"

func Test_newAnnotations(t *testing.T) {
	testCases := []struct {
		prefix              string
		expectedBackendType string
		hasError            bool
	}{
		{
			prefix:              "flannel.alpha.coreos.com",
			expectedBackendType: "flannel.alpha.coreos.com/backend-type",
		},
		{
			prefix:              "flannel.alpha.coreos.com/",
			expectedBackendType: "flannel.alpha.coreos.com/backend-type",
		},
		{
			prefix:              "flannel.alpha.coreos.com/prefix",
			expectedBackendType: "flannel.alpha.coreos.com/prefix-backend-type",
		},
		{
			prefix:              "flannel.alpha.coreos.com/prefix-",
			expectedBackendType: "flannel.alpha.coreos.com/prefix-backend-type",
		},
		{
			prefix:              "org.com",
			expectedBackendType: "org.com/backend-type",
		},
		{
			prefix:              "org9.com",
			expectedBackendType: "org9.com/backend-type",
		},
		{
			prefix:              "org.com/9",
			expectedBackendType: "org.com/9-backend-type",
		},
		{
			// Not a fqdn.
			prefix:   "org",
			hasError: true,
		},
		{
			// Not a fqdn before /.
			prefix:   "org/",
			hasError: true,
		},
		{
			// Not a fqdn before /.
			prefix:   "org/prefix",
			hasError: true,
		},
		{
			// Uppercase letters.
			prefix:   "org.COM",
			hasError: true,
		},
		{
			// Uppercase letters.
			prefix:   "org.com/PREFIX",
			hasError: true,
		},
	}

	for i, tc := range testCases {
		as, err := newAnnotations(tc.prefix)

		if err != nil && !tc.hasError {
			t.Errorf("#%d: error = %s, want nil", i, err)
			continue
		}
		if err == nil && tc.hasError {
			t.Errorf("#%d: error = nil, want non-nil", i)
			continue
		}

		if as.BackendType != tc.expectedBackendType {
			t.Errorf("#%d: BackendType = %s, want %s", i, as.BackendType, tc.expectedBackendType)
		}
	}
}
