## AI-Assisted Workflow: Go/Kubernetes Engineering SOP

This document outlines the standard operating procedure for file analysis and modification. The core principle is a division of labor:
- **Gemini**: Performs all heavy-lifting tasks like analysis, summarization, and planning for large or complex files.
- **Decision-Making Agent (Claude)**: Synthesizes Gemini's output, formulates an action plan, and executes the final response.

This workflow ensures accuracy and efficiency, especially with large files where important details can be missed.

---

### SOP 1: Go/Kubernetes File Analysis

This procedure is mandatory for all file interactions.

**Step 1: Assess File Size**
Before reading any file, check its line count.
```bash
line_count=$(wc -l < "$file" | tr -d ' ')
if [ "$line_count" -gt 300 ]; then
    echo "Large file detected ($line_count lines). Delegating to Gemini for analysis."
fi
```

**Step 2: Delegate Large File Analysis to Gemini**
For any file exceeding 300 lines, use the Gemini CLI to perform the initial analysis. Do not analyze it manually.

**Prompt Template for General Analysis:**
```go
Analyze the Go file '$file' in the context of a Kubernetes project. Provide a structured summary covering:
1.  **Core Purpose**: What is the primary function of this file? (e.g., CRD type definition, controller logic, client-go interaction, storage backend).
2.  **Key Go Components**: Identify major structs, interfaces, functions, and methods. Describe their roles.
3.  **Kubernetes Patterns**: Does this file use client-go, informers, listers, workqueues, or other common Kubernetes patterns? If so, how?
4.  **Dependencies**: List the key Go modules and imported packages. Highlight any dependencies on Kubernetes API groups (e.g., `k8s.io/api/core/v1`, `spdx.softwarecomposition.kubescape.io/v1beta1`).
5.  **Potential Issues**: Flag any potential Go- or Kubernetes-specific issues: race conditions, incorrect use of contexts, inefficient client-go usage, error handling omissions, or anti-patterns.
6.  **Relevance to Request**: How does this file relate to the user's request: "[Insert User's Request Here]"?

Conclude your analysis with 'GEMINI_ANALYSIS_COMPLETE'.
```

**Step 3: Synthesize Gemini's Analysis**
Once Gemini's analysis is complete, the decision-making agent (Claude) must:
1.  Read the full analysis from Gemini.
2.  Correlate the findings with the user's original request.
3.  Identify the key pieces of information needed to formulate a response.

**Step 4: Formulate and Execute Action Plan**
Based on the synthesis, the agent will decide on the final course of action, which may include generating code, creating diffs, or providing a textual explanation. All final outputs must be based on the insights provided by Gemini's analysis.

---

### SOP 2: Complex Refactoring (e.g., File Splitting)

For complex tasks that involve structural changes to the codebase, Gemini must be used to create a detailed implementation plan. Manual planning is forbidden.

**Step 1: Delegate Planning to Gemini**
Generate a plan using a detailed prompt.

**Prompt Template for File Splitting:**
```
Create a detailed plan to split the file '$file' into logical components.

**Context:** The goal is to improve modularity and maintainability.

**Requirements:**
1.  **Component Breakdown**: Propose a set of new files with clear, single-responsibility purposes.
2.  **Content Mapping**: Specify which classes, functions, or code blocks will move to each new file.
3.  **Dependency Management**: Detail how to manage imports/exports to maintain functionality.
4.  **Refactoring Steps**: Provide a step-by-step guide to perform the split without breaking the code.
5.  **Validation**: Suggest how to verify the refactoring was successful.

Conclude your plan with 'GEMINI_PLAN_COMPLETE'.
```

**Step 2: Review and Execute the Plan**
The decision-making agent will review the plan from Gemini. If the plan is sound, the agent will execute it by generating the necessary file modifications or new files as diffs. If the plan is flawed, the agent should prompt Gemini for a revised plan with specific feedback.