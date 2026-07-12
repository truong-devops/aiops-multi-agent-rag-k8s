# admin-web

Next.js operations dashboard for the video/livestream product and AIOps RCA demo.

The UI is intentionally shaped like an internal SaaS control plane for Admin/SRE work: compact navigation, dense tables, restrained colors, semantic status states and toolbar-first workflows. It is not a marketing or landing page.

## Current Screens

- Operations overview.
- Compact KPI strip for gateway, ready videos, processing videos, failures and live sessions.
- Service health/readiness list.
- Recent feed/events and operational queue.
- Feed preview.
- Videos table with search/status/owner filters.
- Video upload drawer using the existing upload-intent and presigned-upload flow.
- Live sessions table with create/start/end actions.
- AIOps incident form, incident placeholders and RCA readiness checklist.
- Account screen for register/login and session token state.

## Development

```bash
cd apps/admin-web
nvm use
cp .env.example .env.local
npm install
npm run dev
```

By default the app calls the product API gateway at `http://localhost:8080`.

```env
NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
```

The app targets Node.js `20.19.0+` because current Next.js security fixes require Node 20 or newer.

## Verification

```bash
npm run lint
npm run typecheck
npm run build
npm audit --omit=dev
```

## Role In Demo

Admin web connects the product flow and AIOps story: it observes video upload/processing, feed readiness, live sessions, service health, incidents, and RCA workflows through the API gateway only.

Known backend caveat: incident/RCA routes are still an evolving backend surface, so the UI keeps those sections ready without pretending every RCA action is fully implemented.
