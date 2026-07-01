# Adiom Crew — Demo App Build Brief

**Purpose:** a small but *production-grade* B2B SaaS that serves as the canvas for a 2-minute Adiom Crew demo.

**Working title:** *On-board*

---

## What it is
A **partner / client onboarding & management portal**. A company's ops/CS team onboards partners (or clients/members), manages them, and sees a dashboard. Deliberately vertical-neutral — it stands in for the "onboarding + dashboards + integrations" problem nearly every prospect on our list has (marketplaces, healthtech, fintech, agencies, edtech).

## Users & roles
- **Admin** (company ops/CS) — primary. Logs in, manages partners, sees the dashboard.
- **Partner** (light/optional) — fills out a public onboarding form; optional read-only status page.

## Data model
- **Partner**: id, name, contact_email, company, region, tier (Starter/Pro/Enterprise), status (Pending/Active/Churned), billing_status, created_at, notes
- **Activity** (optional, for realism): id, partner_id, type, message, created_at

## Screens
1. **Auth** — real admin login
2. **Dashboard** — summary cards (total / active / pending) + partners table (name, company, region, tier, status)
3. **Add Partner** — form with validation → writes to DB
4. **Partner detail** — profile + activity log + actions
5. **Bulk import** — CSV upload to onboard many partners (validation + error report)
6. **Public onboarding form** — partner self-submits; lands as "Pending"

## Stack (production-shaped, on purpose)
- Frontend
- Backend
- Database
- Auth
- Tests
- Seed script (~30 realistic partners) so the dashboard looks alive
