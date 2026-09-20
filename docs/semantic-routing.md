# Jev semantic routing

TokenHub can use TypeSafe's Jev model to choose a Provider and upstream model for eligible Chat Completions requests. Jev is an independent decision client; it does not need to be registered as a TokenHub Provider. The selected generation model must already have an active route and a Provider adapter that supports the request protocol.

## Routing behavior

1. TokenHub authenticates and admits the request, applies privacy and guardrail processing, and returns any response-cache hit without calling Jev.
2. Existing project/API key restrictions, scoped policies, health checks, protocol compatibility, priorities, and the base routing algorithm produce the candidate order.
3. For an opted-in model and project, Jev receives eligible candidate capabilities and user-message text. It returns one candidate or `no_preference` through TypeSafe's `Choice` primitive.
4. In `enforce` mode, a valid choice meeting the confidence threshold promotes that Provider/model within the leading route-priority and resource-priority tier. Resource order within that Provider/model and all other candidates remain intact. `shadow` records the suggestion and keeps the original order.
5. Execution and failover use the resulting order. Jev is called at most once per request, including when upstream execution fails over. No Jev retries are made on the request path.

The six base strategies remain available. A strict primary/backup policy normally has just one candidate in its leading tier, so Jev has nothing to reorder. The routing-policy simulator does not call Jev; it continues to explain the base policy.

## Enable and roll back

The feature defaults to off. Configure the server with:

```dotenv
TOKENHUB_SEMANTIC_ROUTING_ENABLED=true
TOKENHUB_SEMANTIC_ROUTING_PROJECTS=prj_example
TOKENHUB_TYPESAFE_API_KEY=<server-side-secret>
TOKENHUB_TYPESAFE_MODEL=jev-1.13.0
TOKENHUB_SEMANTIC_ROUTING_TIMEOUT_MS=1000
```

The project list is a comma-separated allowlist of exact project IDs. An empty list denies all external routing calls, and startup validation rejects an enabled deployment without a key or project list. Use a concrete evaluator model version; unexpected response versions are rejected. The timeout covers catalog lookup and evaluation; values above 10000 ms or malformed values are rejected. Each process permits at most eight simultaneous Jev calls; excess requests immediately retain the base order.

In **Routing → Model routing policy → Jev Smart Routing**, select:

- **Off**: no evaluation or text egress.
- **Observe** (`shadow`): evaluate and audit, keeping the base order.
- **Enable** (`enforce`): apply an eligible suggestion at or above the threshold.

The initial threshold is `0.65`, a starting value for evaluation, not a validated quality guarantee. Jev confidence describes its choice distribution, not the probability that the chosen model will complete the task successfully. Compare shadow decisions against labeled workload examples before enabling enforcement.

Disable an individual model by selecting **Off**. Set `TOKENHUB_SEMANTIC_ROUTING_ENABLED=false` and restart to disable the feature globally. Neither operation removes routes or changes their base strategy.

## Candidate metadata and public model aliases

Candidates are grouped by the exact Provider ID and upstream model, so multiple accounts in one resource pool do not receive duplicate choices. Only the current public model's admitted routes are eligible; Jev cannot add a Provider, select an arbitrary model, or bypass access restrictions.

To choose across different generation models, explicitly create a public model alias such as `auto-chat` and map its routes to the approved upstream models. Existing public model names never gain extra routes automatically. Existing public-model pricing and billing semantics continue to apply; configuring a multi-model alias does not automatically reprice it per selected upstream model.

Jev receives the provider type, upstream model name, capabilities, input modalities, supported parameters, and the optional `ProviderModel.metadata.routing_description`. Populate that description with verified task suitability, such as evaluated coding or translation strengths. Credentials, endpoints, project names, arbitrary metadata, prices, and operational health records are excluded. Jev is instructed not to invent brand rankings, benchmark scores, or prices. This feature cannot establish which model is best without relevant capability evidence and workload evaluation.

Known catalog text modalities, context limits, and explicit parameter support constrain semantic candidates. Missing catalog entries cannot be promoted. Missing capability declarations do not establish a performance advantage. Metadata over 2048 bytes per candidate is excluded; more than 32 distinct Provider/model pairs disables evaluation for that request.

## Request scope and data handling

This first version supports pure-text `/v1/chat/completions`, including streaming. It skips tools, structured-output constraints, multimodal messages, reasoning continuations, session identifiers, cache affinity, sticky routes, and unknown request/message fields. Responses, Anthropic Messages, embeddings, image APIs, and route simulation keep their existing behavior.

Both **Observe** and **Enable** send processed user-message text and the limited candidate metadata to `https://api.typesafe.ai/v1/systemone`. System/developer messages, assistant messages, headers, API keys, and provider credentials are not sent. User text must fit an 8192-byte budget including separators; larger requests retain the base order instead of being truncated. Restrict the project allowlist to workloads approved for this external service. User text and capability descriptions are treated as data, while local validation enforces the candidate boundary.

Timeouts, cancellation, concurrency saturation, rate limits, overload, malformed/oversized replies, unknown choices, low confidence, and `no_preference` retain the base order. Redirects are not followed. The TypeSafe endpoint is fixed and authentication stays server-side.

Audit events use action `routing.semantic` and the gateway request ID. They record mode, outcome, prompt version, evaluator version, confidence, selected local route IDs, evaluator token counts, and decision latency. They do not contain prompts, response bodies, or credentials. Token counts describe the extra evaluator call; they are not added to the generation model's token usage or customer bill. Accounts must budget the TypeSafe charge separately. Configuration-disabled or disallowed-project requests do not emit semantic audit events.

## API compatibility

The existing `PATCH /api/admin/model-routing-policies/{model}` accepts an optional field alongside the unchanged `strategy` and complete `routes` list:

```json
{
  "semantic_routing": {
    "mode": "shadow",
    "min_confidence": 0.65
  }
}
```

The snippet shows only the added field; a valid PATCH still supplies the base strategy and every route. Mode and an explicit finite confidence threshold from 0 to 1 are required. Invalid settings or routes roll back the entire update. Omitting `semantic_routing` preserves the saved setting for older clients. The policy is stored in `Model.metadata.tokenhub_semantic_routing`; no new database table or migration is required.

## Validation and evaluation

Automated tests use synthetic requests, in-process HTTP servers, and fake generation providers. They cover the TypeSafe wire contract, bounded choices, response validation, modes, eligibility, confidence boundaries, resource pools, permissions, request cancellation, timeouts, failover, atomic persistence, and UI save/reload behavior. They do not measure real workload accuracy or production latency.

For a staged rollout, first use **Observe**, inspect audit outcomes and added latency, and evaluate task success, generation cost, and evaluator cost on representative approved examples. Advance to **Enable** only for models and projects whose evaluation supports that choice.

Protocol references: [TypeSafe API](https://docs.typesafe.ai/api), [Choice](https://docs.typesafe.ai/primitives/choice), and [Confidence](https://docs.typesafe.ai/confidence).

An optional real API contract smoke is available as `TestJevRoutingClientLive`. Set `TOKENHUB_LIVE_TYPESAFE_API_KEY` only in the test process environment and run `go test ./internal/server -run ^TestJevRoutingClientLive$ -count=1 -v` from `backend/`. It sends a synthetic translation request to TypeSafe and calls no generation Provider; by default it is skipped.

The semantic metadata field is managed by the routing-policy endpoint for existing models. Ordinary model edits, stale metadata payloads, and startup catalog refreshes preserve the current saved setting. Model and policy writers lock the model before its routes so a concurrent edit cannot restore an older external-routing setting.
