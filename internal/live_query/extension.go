// Copyright 2023 Upbound Inc
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

package live_query

import (
	"context"
	"fmt"
	"time"

	"github.com/99designs/gqlgen/graphql"
	jd "github.com/josephburnett/jd/lib"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const (
	// extName is the name of the extension
	extName = "LiveQuery"
	// fieldName is the field name exposing live queries.
	fieldName = "liveQuery"
	// typeQuery is the schema type that will be made subscribeable.
	typeQuery = "Query"
	// typeSubscription is the schema type that will expose "liveQuery" field.
	typeSubscription = "Subscription"
	// argThrottle is the name of the "trottle" argument for the "liveQuery" field.
	argThrottle = "throttle"
)

var _ interface {
	graphql.HandlerExtension
	graphql.OperationInterceptor
	graphql.OperationParameterMutator
	graphql.OperationContextMutator
} = LiveQuery{}

// LiveQuery is a graphql.HandlerExtension that enables live queries.
type LiveQuery struct{}

// ExtensionName implements graphql.HandlerExtension
func (LiveQuery) ExtensionName() string {
	return extName
}

// Validate implements graphql.HandlerExtension
func (l LiveQuery) Validate(s graphql.ExecutableSchema) error {
	subscriptionType, ok := s.Schema().Types[typeSubscription]
	if !ok {
		return fmt.Errorf("%q type not found", typeSubscription)
	}

	field := subscriptionType.Fields.ForName(fieldName)
	if field == nil {
		return fmt.Errorf("%q type is missing %q field", typeSubscription, fieldName)
	}
	if field.Type.String() != typeQuery {
		return fmt.Errorf("%q field on %q is not of type %q", fieldName, typeSubscription, typeQuery)
	}
	if field.Arguments.ForName(argThrottle) == nil {
		return fmt.Errorf("%q field on %q is missing the %q argument", fieldName, typeSubscription, argThrottle)
	}

	return nil
}

type patch struct {
	Revision  int         `json:"revision"`
	JSONPatch []Operation `json:"jsonPatch,omitempty"`
}

type LiveQueryStats struct {
	Revisions map[string]int         `json:"revision"`
	PrevData  map[string]jd.JsonNode `json:"prevData"`
	Throttle  int                    `json:"throttle,omitempty"`
}

func (l LiveQuery) MutateOperationParameters(ctx context.Context, request *graphql.RawParams) *gqlerror.Error {
	return nil
}

// MutateOperationContext implements graphql.OperationContextMutator
func (l LiveQuery) MutateOperationContext(ctx context.Context, rc *graphql.OperationContext) *gqlerror.Error {
	// we're only interested in subscriptions
	if rc.Operation.Operation != ast.Subscription {
		return nil
	}
	fields := graphql.CollectFields(rc, rc.Operation.SelectionSet, []string{typeSubscription})
	if len(fields) != 1 {
		return nil
	}
	// check that the subscription field is "liveQuery"
	field := fields[0]
	if field.Name != fieldName {
		return nil
	}
	operationCopy := *rc.Operation
	operationCopy.Operation = ast.Query
	operationCopy.SelectionSet = field.SelectionSet
	rc.Operation = &operationCopy
	rc.Stats.SetExtension(extName, &LiveQueryStats{
		Throttle:  (int)(field.ArgumentMap(rc.Variables)["throttle"].(int64)),
		Revisions: make(map[string]int),
		PrevData:  make(map[string]jd.JsonNode),
	})
	return nil
}

// InterceptOperation implements graphql.OperationInterceptor
func (l LiveQuery) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler { //nolint:gocyclo
	oc := graphql.GetOperationContext(ctx)
	lqs, ok := oc.Stats.GetExtension(extName).(*LiveQueryStats)
	if !ok {
		return next(ctx)
	}
	throttle := time.Duration(lqs.Throttle) * time.Millisecond
	lq, ctx := withLiveQuery(ctx, throttle)
	handler := next(ctx)
	return func(ctx context.Context) *graphql.Response {
		for {
			// create the handler when live query is ready.
			if handler == nil {
				select {
				case <-lq.Ready():
					handler = next(ctx)
				case <-ctx.Done():
					return nil
				}
			}
			resp := handler(ctx)
			// reached the end of the handler, including deferreds.
			if resp == nil {
				// reset live query and handler for waiting.
				handler = nil
				lq.Reset()
				continue
			}
			// propagate errors
			data, err := jd.ReadJsonString(string(resp.Data))
			if err != nil {
				panic(err)
			}
			if prevData, ok := lqs.PrevData[resp.Path.String()]; ok {
				diff, err := CreateJSONPatch(prevData, data)
				if err != nil {
					panic(err)
				}
				if len(diff) > 0 {
					// reset data and add patch extension.
					resp.Data = nil
					resp.Extensions["patch"] = patch{
						Revision:  lqs.Revisions[resp.Path.String()],
						JSONPatch: diff,
					}
				} else if len(resp.Errors) == 0 {
					// nothing changed, wait for next change.
					continue
				}
			}
			lqs.Revisions[resp.Path.String()] += 1
			// keep current data as previous response.
			lqs.PrevData[resp.Path.String()] = data
			return resp
		}
	}
}
