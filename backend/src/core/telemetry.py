from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import (
    BatchSpanProcessor,
    ConsoleSpanExporter,
    SimpleSpanProcessor,
)
from opentelemetry.sdk.resources import Resource

def setup_telemetry(service_name: str = "rice-search-backend"):
    """
    Configures OpenTelemetry to export traces to console.
    """
    resource = Resource.create({"service.name": service_name})
    provider = TracerProvider(resource=resource)
    
    # Export to Console for MVP (visible in Docker logs)
    # Use SimpleSpanProcessor for immediate output in dev/debug
    processor = SimpleSpanProcessor(ConsoleSpanExporter())
    provider.add_span_processor(processor)
    
    # Sets the global default tracer provider
    trace.set_tracer_provider(provider)
    
    return provider
