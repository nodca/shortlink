package trace

// OpenTelemetry GenAI semantic convention keys (subset).
// Spec: https://opentelemetry.io/docs/specs/semconv/gen-ai/
const (
	GenAISystem        = "gen_ai.system"
	GenAIRequestModel  = "gen_ai.request.model"
	GenAIUsageInput    = "gen_ai.usage.input_tokens"
	GenAIUsageOutput   = "gen_ai.usage.output_tokens"
	GenAIAgentStepKind = "gen_ai.agent.step"
)
