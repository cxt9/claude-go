---
description: "This rule provides standards for design log files"
alwaysApply: true
---

# Claude Go Project Rules

## Design Log Methodology

The project follows a rigorous design log methodology for all significant features and architectural changes.

### Before Making Changes
1. Check design logs in `./design-log/` for existing designs and implementation notes
2. For new features: Create design log first, get approval, then implement
3. Read related design logs to understand context and constraints
4. Use relevant available agents when needed:

  | Agent                                            | Description                                                                                                                |
  |--------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------|
  | general-purpose                                  | General-purpose agent for researching complex questions, searching for code, and executing multi-step tasks                |
  | Explore                                          | Fast agent for exploring codebases - find files by patterns, search code for keywords, answer questions about the codebase |
  | Plan                                             | Software architect agent for designing implementation plans, step-by-step plans, identifies critical files                 |
  | claude-code-guide                                | Answers questions about Claude Code CLI, Agent SDK, and Claude API usage                                                   |
  | code-documentation:code-reviewer                 | Code review expert - security vulnerabilities, performance optimization, production reliability                            |
  | code-documentation:docs-architect                | Creates comprehensive technical documentation from existing codebases                                                      |
  | code-documentation:tutorial-engineer             | Creates step-by-step tutorials and educational content from code                                                           |
  | backend-development:backend-architect            | Scalable API design, microservices architecture, distributed systems                                                       |
  | backend-development:event-sourcing-architect     | Event sourcing patterns and architecture                                                                                   |
  | backend-development:graphql-architect            | GraphQL federation, performance optimization, enterprise security                                                          |
  | backend-development:tdd-orchestrator             | TDD orchestrator - red-green-refactor discipline, test-driven development                                                  |
  | backend-development:temporal-python-pro          | Temporal workflow orchestration with Python SDK                                                                            |
  | multi-platform-apps:flutter-expert               | Flutter development with Dart 3, multi-platform deployment                                                                 |
  | multi-platform-apps:frontend-developer           | React 19, Next.js 15, modern frontend architecture                                                                         |
  | multi-platform-apps:ios-developer                | Native iOS with Swift/SwiftUI, iOS 18                                                                                      |
  | multi-platform-apps:mobile-developer             | React Native, Flutter, native mobile apps                                                                                  |
  | multi-platform-apps:ui-ux-designer               | Interface designs, wireframes, design systems                                                                              |
  | frontend-mobile-development:frontend-developer   | React components, responsive layouts, client-side state                                                                    |
  | frontend-mobile-development:mobile-developer     | Cross-platform mobile development                                                                                          |
  | code-refactoring:legacy-modernizer               | Refactor legacy codebases, migrate frameworks, reduce technical debt                                                       |
  | dependency-management:legacy-modernizer          | Framework migrations, dependency updates                                                                                   |
  | agent-orchestration:context-manager              | AI context engineering, vector databases, knowledge graphs                                                                 |
  | full-stack-orchestration:deployment-engineer     | CI/CD pipelines, GitOps, deployment automation                                                                             |
  | full-stack-orchestration:performance-engineer    | Observability, optimization, OpenTelemetry, load testing                                                                   |
  | full-stack-orchestration:security-auditor        | DevSecOps, vulnerability assessment, OWASP, compliance                                                                     |
  | full-stack-orchestration:test-automator          | AI-powered test automation, quality engineering                                                                            |
  | security-scanning:security-auditor               | Security audits, threat modeling, compliance                                                                               |
  | security-scanning:threat-modeling-expert         | Threat modeling expertise                                                                                                  |
  | api-scaffolding:backend-architect                | API design, microservices                                                                                                  |
  | api-scaffolding:django-pro                       | Django 5.x with async views, DRF, Celery                                                                                   |
  | api-scaffolding:fastapi-pro                      | FastAPI, SQLAlchemy 2.0, Pydantic V2                                                                                       |
  | api-scaffolding:graphql-architect                | GraphQL architecture                                                                                                       |
  | api-testing-observability:api-documenter         | OpenAPI 3.1, API documentation, developer portals                                                                          |
  | backend-api-security:backend-security-coder      | Secure backend coding, input validation, API security                                                                      |
  | frontend-mobile-security:frontend-security-coder | XSS prevention, client-side security                                                                                       |
  | frontend-mobile-security:mobile-security-coder   | Mobile security, WebView security                                                                                          |


### When Creating Design Logs
1. Structure: Background → Problem → Questions and Answers → Design → Implementation Plan → Examples → Trade-offs
2. Be specific: Include file paths, type signatures, validation rules
3. Show examples: Use ✅\❌ for good/bad patterns, include realistic code
4. Explain why: Don't just describe what, explain rationale and trade-offs
5. Ask Questions (in the file): For anything that is not clear, or missing information
6. When answering question: keep the questions, just add answers
7. Be brief: write short explanations and only what most relevant
8. Draw Diagrams: Use mermain inline diagrams when it makes sense

### When Implementing
1. Follow the implementation plan phases from the design log
2. Write tests first or update existing tests to match new behavior
3. Do not Update design log initial section once implementation started
4. Append design log with "Implementation Results" section as you go
5. Document deviations: Explain why implementation differs from design
6. Run tests: Include test results (X/Y passing) in implementation notes
7. After Implementation add a summary of deviations from original design

### When Answering Questions
1. Reference design logs by number when relevant (e.g., "See Design Log #50")
2. Use codebase terminology: ViewState, Contract, JayContract, phase annotations
3. Show type signatures: This is a TypeScript project with heavy type usage
4. Consider backward compatibility: Default to non-breaking changes