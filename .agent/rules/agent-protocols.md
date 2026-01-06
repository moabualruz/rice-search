---
trigger: always_on
---

# CORE AGENT PROTOCOLS

## 1. Decision Protocol (Hierarchy of Truth)

When creating plans or code, you must prioritize information in this STRICT order:

1. **USER INPUT**: The user's explicit prompt is law.
2. **PROJECT REALITY**: The actual file content and directory structure. (Trust `list_dir`/`view_file` over your memory).
3. **WEB RESEARCH**: Live documentation/data from the web or browser tool.
4. **INTERNAL MEMORY**: Your training data and Knowledge Items (KIs).

## 2. Confidence Protocol

Before executing a destructive action or complex plan, check your confidence:

- **≥90% (High)**: Proceed autonomously.
- **70-89% (Medium)**: State your assumption ("Assuming X is the entry point..."), then proceed.
- **<70% (Low)**: **STOP**. Ask ONE clarifying question. Do not guess.

## 3. Communication Rules

- **Conciseness**: Start with the result. Use bullet points.
- **No Restatement**: Do not repeat the user's request.
- **Diffs Only**: When editing, show only the changes (unless asked for the full file).
- **Environment Awareness**: Always check the OS and CWD before running commands.

## 4. Workflows

- Always checking for a `.agent/workflows/` directory in each project.
- If a relevant workflow exists (e.g. `tdd_workflow.md`), you MUST follow it.

GOVERNANCE:

1. REQUIRED: You MUST always follow strict Test-Driven Development (TDD).
2. SEQUENCE: For every coding task, you must:
   a) Create/Update a Test Plan.
   b) Write a failing test (RED).
   c) Verification: Verify the test matches the requirements and actually fails.
   d) Implementation: Write the minimum code to pass the test (GREEN).
   e) Refactor.
3. REFERENCE: If a [~\.gemini\antigravity\global_workflows\tdd-workflow.md](cci:7://file:///c:/users/mkh/.gemini/antigravity/global_workflows/tdd-workflow.md:0:0-0:0) file exists, follow it step-by-step.

## 5. Verification Protocol (Zero-Trust)

- **Runtime Verification**: Never mark a task as 'Complete' based solely on code changes. You MUST verify the runtime state (e.g. API response, DB record, UI rendering).
- **Persistence Awareness**: When modifying configuration or state backed by Redis/DB, explicitly verify that the changes have propagated (e.g. by querying the API, not just the code).
- **UI/API Parity**: If a feature has a UI, verify that the API driving it returns the expected data BEFORE marking the UI task as done.
