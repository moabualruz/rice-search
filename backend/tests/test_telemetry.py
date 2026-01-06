import pytest
from unittest.mock import patch, MagicMock
from fastapi import FastAPI
from src.core.telemetry import setup_telemetry

def test_setup_telemetry_instrumentation():
    """Verify that FastAPI app is instrumented"""
    app = FastAPI()
    
    with patch("src.core.telemetry.FastAPIInstrumentor") as mock_instrumentor, \
         patch("src.core.telemetry.TracerProvider") as mock_provider, \
         patch("src.core.telemetry.BatchSpanProcessor") as mock_processor, \
         patch("src.core.telemetry.OTLPSpanExporter") as mock_exporter, \
         patch("src.core.telemetry.trace") as mock_trace:
        
        setup_telemetry(app, "test-service", "http://jaeger:4317")
        
        # Should verify these are called
        mock_instrumentor.instrument_app.assert_called_once_with(app)
        mock_provider.assert_called()
        mock_trace.set_tracer_provider.assert_called()
