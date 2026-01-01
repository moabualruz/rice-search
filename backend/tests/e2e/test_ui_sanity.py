import pytest
import os
from playwright.sync_api import Page, expect

@pytest.mark.e2e
def test_frontend_loads(page: Page):
    """
    Sanity check: Frontend is accessible and renders title.
    """
    # Default to 'http://frontend:3000' for Docker, 'http://localhost:3000' for local
    base_url = os.getenv("FRONTEND_URL", "http://frontend:3000")
    
    try:
        page.goto(base_url, timeout=10000) # 10s timeout
    except Exception as e:
        pytest.fail(f"Failed to load frontend at {base_url}: {e}")

    # Check title
    # Note: You might need to adjust this depending on actual metadata
    try:
        expect(page).to_have_title("Rice Search")
    except AssertionError:
        # Fallback check for text content if title is dynamic/missing
        expect(page.locator("body")).to_contain_text("Rice Search")
