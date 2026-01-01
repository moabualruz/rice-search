from fastapi import FastAPI
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.requests import RequestsInstrumentor

def setup_telemetry(app: FastAPI, service_name: str, endpoint: str):
    """
    Sets up OpenTelemetry tracing with OTLP exporter if enabled.
    """
    if not endpoint:
        return

    # Create Resource
    resource = Resource(attributes={
        "service.name": service_name
    })

    # Setup Provider
    provider = TracerProvider(resource=resource)
    trace.set_tracer_provider(provider)

    # Setup Exporter (OTLP gRPC)
    exporter = OTLPSpanExporter(endpoint=endpoint, insecure=True)
    processor = BatchSpanProcessor(exporter)
    provider.add_span_processor(processor)

    # Instrument FastAPI
    FastAPIInstrumentor.instrument_app(app)
    
    # Instrument Requests
    RequestsInstrumentor().instrument()
