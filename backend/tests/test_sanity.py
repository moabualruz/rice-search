
def test_import_ingestion():
    try:
        from src.tasks import ingestion
    except ImportError as e:
        pytest.fail(f"Import failed: {e}")
