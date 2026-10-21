---
name: sre-orchestrator
description: Use when developing, architecting, debugging, or extending the sre-network-discovery project to automatically orchestrate Graphify architecture analysis, Superpowers structured workflow (brainstorming, plans, TDD), Ponytail minimalist code standards, and native/web interface plugins.
---

# SRE Network Discovery Orchestrator

This skill coordinates and automates the full engineering pipeline for the **sre-network-discovery** project by seamlessly chaining together installed system plugins and tools: **Graphify**, **Superpowers**, **Ponytail**, and specialized language/UI plugins (**Web/TS/JS**, **Native Systems C++/C#**, and **App Interfaces**).

---

## 1. Automatic Execution Pipeline

Every task in `sre-network-discovery` must follow this 5-stage automated workflow:

```
[Phase 1: Architecture & Topology (Graphify)]
                  │
                  ▼
[Phase 2: Clarification & Planning (Superpowers)]
                  │
                  ▼
[Phase 3: Minimal Implementation & TDD (Ponytail + Superpowers)]
                  │
                  ▼
[Phase 4: Systematic Verification & Debugging (Superpowers)]
                  │
                  ▼
[Phase 5: Code Review & Delivery (Ponytail Review + Superpowers)]
```

---

## Phase 1: Context & Architecture Mapping (Graphify)

Before modifying code or planning architecture:
1. **Check for Knowledge Graph**:
   - Verify if `graphify-out/graph.json` exists in the workspace.
   - If missing or after significant structural changes, run:
     ```bash
     graphify .
     ```
2. **Query Architectural Relationships**:
   - Use `graphify query "<component or protocol>"` to trace dependencies (e.g., packet collectors, scanner workers, topology visualizer, alert triggers).
   - Use `graphify path "<source>" "<target>"` to understand data flow between network discovery probes and persistence/UI layers.
   - Use Graphify MCP tools (`mcp_graphify`) if active.

---

## Phase 2: Specification & Planning (Superpowers)

1. **Brainstorm & Clarify (`brainstorming`)**:
   - For any new discovery protocol (e.g., ARP, ICMP, SNMP, LLDP, mDNS, eBPF) or UI component:
     - Clarify network requirements, timeout handling, concurrency limits, and permissions (e.g., raw socket capabilities vs. unprivileged scans).
2. **Implementation Plan (`writing-plans`)**:
   - Break down the task into bite-sized, verifiable units:
     - Component interfaces and domain models.
     - Unit/integration test scenarios.
     - Concrete verification commands.

---

## Phase 3: Minimal & Resilient Implementation (Ponytail + Superpowers)

1. **Apply Ponytail Decision Ladder (YAGNI & Anti-Bloat)**:
   - **Step 1**: Does this feature strictly need to exist? Avoid premature abstractions.
   - **Step 2**: Can we reuse existing internal helpers and structures?
   - **Step 3**: Does the standard library / OS runtime provide it natively?
     - Networking: Prefer native socket abstractions (`socket`, `asyncio`, `std::jthread`, `System.Threading.Channels`) before adding heavy external dependencies.
   - **Step 4**: Smallest, most readable solution wins.
2. **Test-Driven Development (`test-driven-development`)**:
   - Write failing unit/integration tests for packet encoders, parser boundaries, and scanner limits first (RED).
   - Implement minimal working code to pass tests (GREEN).
   - Refactor cleanly without adding unnecessary complexity (REFACTOR).

---

## Phase 4: Language & Interface Domain Specialization

Invoke the appropriate installed plugin skills based on the subsystem:

- **Web UI & Dashboards (Network Visualizer, Topology Canvas, Real-time Alerts)**:
  - Use `web-interface-engineering` and `frontend-design` for responsive dashboards, SVG/Canvas network graph visualizers, and accessible controls.
  - Use `typescript-javascript-mastery` for strict type contracts, Zod schema validation of incoming network probe metrics, and WebSocket streaming.
  - Use `webapp-testing` and `chrome-devtools` for headless browser and performance validation.

- **Native Desktop & Mobile Interfaces (Operator Consoles, Field Tools)**:
  - Use `app-interface-architecture` for MVVM decoupling, background scanner thread isolation, and high-frequency UI updates.
  - Use `flutter` or `android-cli-plugin` when target encompasses mobile/embedded field diagnostic tools.

- **High-Performance Network Core (Scanners, Packet Ingestion, SRE Probes)**:
  - Use `cpp-modern-architecture` for C++20 raw socket filters, zero-copy packet rings, and RAII resource management.
  - Use `csharp-dotnet-mastery` for .NET 8/9 async channels, `ReadOnlySpan<byte>` packet parsing, and cross-platform network services.

---

## Phase 5: Verification, Review & Delivery

1. **Systematic Debugging (`systematic-debugging`)**:
   - If packet drop, socket leaks, concurrency race conditions, or parsing crashes occur:
     - Formulate hypothesis -> isolate root cause with minimal reproducible test -> fix -> re-verify. Never patch symptomatically.
2. **Verification Before Completion (`verification-before-completion`)**:
   - Run complete test suite, linters, and build checks.
   - Provide concrete command output proof before stating task is complete.
3. **Ponytail Review & Clean-up (`ponytail-review` & `finishing-a-development-branch`)**:
   - Review git diff to ensure no leftover debug code, excessive abstractions, or unnecessary dependencies.
   - Update `GRAPH_REPORT.md` / `graphify --update` if codebase topology changed.

