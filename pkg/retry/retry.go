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
	log "k8s.io/klog"

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
