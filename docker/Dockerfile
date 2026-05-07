# Copyright 2026 The Volcano Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# ---- Build stage ----
FROM golang:1.25 AS builder

WORKDIR /workspace

# Cache dependency downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a fully static binary.
COPY . .

ARG VERSION=unknown
ARG GIT_SHA=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags " \
        -X 'volcano.sh/resource-exporter/pkg/version.Version=${VERSION}' \
        -X 'volcano.sh/resource-exporter/pkg/version.GitSHA=${GIT_SHA}' \
        -X 'volcano.sh/resource-exporter/pkg/version.Built=${BUILD_DATE}'" \
    -o /numatopo ./

# ---- Runtime stage ----
# This container runs as root because it must read kubelet's
# cpu_manager_state file which is owner-only (0600, root:root).
FROM alpine:latest

COPY --from=builder /numatopo /numatopo

ENTRYPOINT ["/numatopo"]
