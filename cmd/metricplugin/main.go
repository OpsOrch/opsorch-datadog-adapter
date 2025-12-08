package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	coremetric "github.com/opsorch/opsorch-core/metric"
	"github.com/opsorch/opsorch-core/schema"
	adapter "github.com/opsorch/opsorch-datadog-adapter/metric"
)

type rpcRequest struct {
	Method  string          `json:"method"`
	Config  map[string]any  `json:"config"`
	Payload json.RawMessage `json:"payload"`
}

type rpcResponse struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

var provider coremetric.Provider

func main() {
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for {
		var req rpcRequest
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			writeErr(enc, err)
			return
		}

		prov, err := ensureProvider(req.Config)
		if err != nil {
			writeErr(enc, err)
			continue
		}

		ctx := context.Background()
		switch req.Method {
		case "metric.query":
			var query schema.MetricQuery
			if err := json.Unmarshal(req.Payload, &query); err != nil {
				writeErr(enc, err)
				continue
			}
			res, err := prov.Query(ctx, query)
			write(enc, res, err)
		case "metric.describe":
			var payload struct {
				Scope schema.QueryScope `json:"scope"`
			}
			if err := json.Unmarshal(req.Payload, &payload); err != nil {
				writeErr(enc, err)
				continue
			}
			res, err := prov.Describe(ctx, payload.Scope)
			write(enc, res, err)
		default:
			writeErr(enc, fmt.Errorf("unknown method: %s", req.Method))
		}
	}
}

func ensureProvider(cfg map[string]any) (coremetric.Provider, error) {
	if provider != nil {
		return provider, nil
	}
	prov, err := adapter.New(cfg)
	if err != nil {
		return nil, err
	}
	provider = prov
	return provider, nil
}

func write(enc *json.Encoder, result any, err error) {
	if err != nil {
		writeErr(enc, err)
		return
	}
	_ = enc.Encode(rpcResponse{Result: result})
}

func writeErr(enc *json.Encoder, err error) {
	_ = enc.Encode(rpcResponse{Error: err.Error()})
}
