# Product Overview: Controls Tracker (mycis)

## What is this application?
**Controls Tracker (mycis)** is a web-based application designed to help organizations and security teams track security control assessments against published cybersecurity frameworks, primarily focusing on frameworks like the CIS (Center for Internet Security) Controls. 

## Purpose
The main purpose of the application is to provide a collaborative, centralized platform where teams can evaluate their organization's security posture and track compliance and implementation of security controls over time. Instead of using complex and error-prone spreadsheets to track progress, this tool allows teams to:
- Open specific assessments tied to a loaded framework.
- Score, prioritize, and track individual security controls (e.g., scores 1–5).
- Assign owners and reviewers to specific controls.
- Set due dates for implementing controls.
- Attach evidence links and leave comments for each control.
- Manage user access, permissions, and assessment lifecycles (via administrative privileges).

## Core Features
1. **Framework Management**: Browse loaded frameworks (like CIS v8.1), view control groups, and read control definitions. Framework data is defined via YAML and seeded into the database directly.
2. **Assessments Dashboard**:
   - Create and manage new assessments tied to a specific framework.
   - Detailed views displaying per-control status, score, priority, ownership, and trackable milestones.
   - Advanced query-based filtering mechanisms for assessment items to easily drill down into specific areas of concern.
   - Bulk update capabilities for items within an assessment.
3. **Authentication & Authorization**:
   - Secure web-based sign-in using email and passwords.
   - Session-based authentication using HTTP-only cookies (via `gorilla/sessions`).
   - Role-based separation: regular users can manage and review controls, while admin users can create overall assessments and manage user accounts.
   - Enforced password changes for newly created or temporary user accounts.
4. **Command-Line Interface (CLI)**:
   - Includes built-in CLI commands in the main binary to run database migrations out-of-the-box (`migrate`), bootstrap the initial administrator user (`create-admin`), and seed standard framework YAML files into the Postgres database (`seed-framework`).

## Technology Stack
The application is built using a modern, efficient, and strongly-typed stack:
- **Backend Execution**: 
  - Written in **Go 1.26**.
  - **Routing/HTTP**: `Echo v5` for routing, middleware, and HTTP request handling.
  - **Database**: PostgreSQL 16 using `pgx/v5` for optimized database driver connectivity.
  - **Data Access**: `sqlc` for type-safe SQL query generation directly generated from raw `.sql` files.
  - **Migrations**: `golang-migrate` for managing database schema changes via plain SQL.
- **Frontend / UI**:
  - **Templates**: Go's native standard library `html/template` package executing server-side templates under the `internal/http/templates` directory.
  - **Styling**: Tailwind CSS v4 and `basecoat-css` for a minimal, clean, utility-first UI design.
  - **Asset Building**: `esbuild` for bundling front-end assets (JS/CSS).
- **Developer Tooling**:
  - Docker and Docker Compose for out-of-the-box local environment setup.
  - `Just` file for task running and `Air` for live-reloading during development.

## What's Next (AI Context)
This document serves as a baseline of the application's current capabilities. Since the foundation is solid and built on Go/Postgres, potential areas for expansion or improvement could include:
- **Reporting**: Generating PDF or CSV exports of assessments for executive summaries.
- **Integrations**: Adding REST or GraphQL API endpoints for automated evidence collection from third-party cloud security tools (e.g., AWS Security Hub, vulnerability scanners, MDM tools).
- **Enterprise Features**: Single Sign-On (SSO) integration using SAML or OIDC.
- **Analytics**: Dashboard visualizations and charting for tracking compliance progress over time across multiple concurrent assessments.
- **Workflow**: Automated notifications (email, Slack, Teams) for overdue controls, mapping custom internal frameworks, or advanced audit logging.
