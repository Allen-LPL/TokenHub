package server

import (
	"context"
	"encoding/json"
	"net/http"
)

func (s *Server) executeRoutedSystemOne(r *http.Request, routed RoutedCall, req SystemOneRequest) (any, RouteSelection, Usage, []RouteAttempt, error) {
	return executeRoutedWithStore(r.Context(), s.store, routed, false, func(ctx context.Context, route RouteSelection, _ bool, _ int) (any, Usage, error) {
		route, err := s.prepareRouteForUpstream(ctx, route)
		if err != nil {
			return nil, Usage{}, err
		}
		// Clone the nested request before a route-specific transform or failover.
		body, err := json.Marshal(req)
		if err != nil {
			return nil, Usage{}, err
		}
		var upstream SystemOneRequest
		if err := json.Unmarshal(body, &upstream); err != nil {
			return nil, Usage{}, err
		}
		if err := s.runGatewayRequestTransformHooks(ctx, routed.Call, route, upstream, providerRouteProtocolSystemOne, func(data json.RawMessage) error {
			return applySystemOneRequestPatch(&upstream, data)
		}); err != nil {
			return nil, Usage{}, err
		}
		if response, usage, handled, err := s.runGatewayProviderCallHooks(ctx, routed.Call, route, upstream, providerRouteProtocolSystemOne); err != nil || handled {
			if err == nil {
				var result SystemOneResponse
				result, err = decodeSystemOneGatewayResponse(response)
				if err == nil {
					// Native usage is required even if the hook omits DataUsage.
					metered := result.meteredUsage()
					metered.UpstreamRequestID = usage.UpstreamRequestID
					metered.ResponseHeaders = usage.ResponseHeaders
					response, usage = result, metered
					err = result.validate(upstream)
				}
			}
			return response, usage, err
		}
		adapter, ok := resolveTypedAdapter[SystemOneInvoker](s.adapterRegistry, route.Provider.Type)
		if !ok {
			return nil, Usage{}, NewHTTPError(http.StatusNotImplemented, "provider_capability_not_supported", "System One is not supported")
		}
		return adapter.SystemOne(ctx, route.Provider, route.ProviderModel, upstream)
	})
}

func validateSystemOneGatewayResponse(response any, req SystemOneRequest) error {
	result, err := decodeSystemOneGatewayResponse(response)
	if err != nil {
		return err
	}
	return result.validate(req)
}

func decodeSystemOneGatewayResponse(response any) (SystemOneResponse, error) {
	if result, ok := response.(SystemOneResponse); ok {
		return result, nil
	}
	data, err := json.Marshal(response)
	if err != nil {
		return SystemOneResponse{}, invalidSystemOneResponse()
	}
	var result SystemOneResponse
	if json.Unmarshal(data, &result) != nil {
		return result, invalidSystemOneResponse()
	}
	return result, nil
}
