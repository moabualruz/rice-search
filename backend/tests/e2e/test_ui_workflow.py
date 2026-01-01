import pytest
import re
from playwright.sync_api import Page, expect
import os

# E2E tests run in a separate container, so they access the API via HTTP
# They cannot directly access DB session of the running API container
# Use page.request or requests to seed data if needed

@pytest.mark.e2e
def test_search_interaction(page: Page):
    """
    Test the Search UI interaction flow.
    Verifies that we can type, switch modes, and submit a search.
    """
    # Environment variables are set in docker-compose
    base_url = os.getenv("FRONTEND_URL", "http://frontend:3000")
    
    # 1. Navigate to Home
    page.goto(base_url)
    
    # Verify Title (Lowercase branding with ?earch)
    # expect(page).to_have_title("rice ?earch") # Exact match check
    expect(page).to_have_title(re.compile(r"rice \?earch", re.IGNORECASE))
    
    # 2. Check Modes
    # Default is RAG "Ask AI"
    rag_btn = page.get_by_role("button", name="Ask AI")
    # Verify it has active class (purple)
    expect(rag_btn).to_have_class(re.compile(r"bg-purple-500"))
    
    # 3. Switch to Search Mode
    search_btn = page.get_by_role("button", name="Search")
    search_btn.click()
    
    # Verify active class changes (indigo)
    expect(search_btn).to_have_class(re.compile(r"bg-indigo-500"))
    
    # Verify placeholder changes
    input_field = page.get_by_placeholder("Search documents...")
    expect(input_field).to_be_visible()
    
    # 4. Perform Search
    input_content = "test query"
    input_field.fill(input_content)
    
    # Submit
    submit_btn = page.locator("button[type='submit']")
    submit_btn.click()
    
    # 5. Verify Loading State or Results
    # Loading spinner usually replaces button content or appears
    # Here we just verify that we are still on the page and UI didn't crash
    # And potentially check for "Results" or "No results" or "Error"
    
    # The frontend shows "Results" header if successful
    # Or console errors if failed
    
    # Wait for response (network idle) or UI update
    # Note: If backend is empty, it might return empty list immediately
    
    # Verify we are on a page that (eventually) shows Results section or Error
    # We can wait for the 'sources' icon (Database icon) or 'FileText' if results found
    # Or just ensure input is still there
    expect(input_field).to_have_value(input_content)
    
    # If the backend is running and empty, we might not see any results cards
    # But we shouldn't see an application error page
    
    # Optional: Check for network errors
    # page.on("console", lambda msg: print(f"Console: {msg.text}"))
