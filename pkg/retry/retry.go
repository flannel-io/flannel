// Copyright 2023 flannel authors
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

package retry

import (
	"context"
	"time"

	log "k8s.io/klog/v2"

	retry_v4 "github.com/avast/retry-go/v4"
)

// Do executes f until it does not return an error
// By default, the number of attempts is 10 with increasing delay between each
func Do(f func() error) error {

	return retry_v4.Do(f,
		retry_v4.OnRetry(func(n uint, err error) {
			log.Errorf("#%d: %s\n", n, err)
		}),
	)

}

func DoUntil(f func() error, ctx context.Context, timeout time.Duration) error {
	toctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return retry_v4.Do(f,
		retry_v4.Context(toctx),
		retry_v4.OnRetry(func(n uint, err error) {
			log.Errorf("#%d: %s\n", n, err)
		}),
		retry_v4.Attempts(0),
		retry_v4.WrapContextErrorWithLastError(true))
}

// DoWithOptions executes f until it does not return an error
// By default, the number of attempts is 10 with increasing delay between each
// It also accept Options from the retry package
func DoWithOptions(f func() error, opts ...retry_v4.Option) error {

	return retry_v4.Do(f, opts...)

}
