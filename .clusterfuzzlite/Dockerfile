# Copyright 2024 flannel authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM gcr.io/oss-fuzz-base/base-builder-go@sha256:da01cfabf7f4b558ce57eedca2209b865b0f070e4ec8e809b137b567e054b318 # latest

# Copy the whole repository into $SRC so build.sh can compile the fuzzers.
COPY . $SRC/flannel
WORKDIR $SRC/flannel
COPY .clusterfuzzlite/build.sh $SRC/build.sh
