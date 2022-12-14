// Copyright 2022 Upbound Inc
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

package internal

// HealthOptions options related to readiness and liveness endpoints.
type HealthOptions struct {
	HealthPort int  `default:"8088" help:"Port for readiness and liveness server."`
	Health     bool `name:"health" default:"true" negatable:"" help:"Enable readiness and liveness endpoints."`
}
