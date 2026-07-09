# admin-web

Next.js admin dashboard for the product and AIOps demo.

## Screens

- Operations overview.
- Service health.
- Videos and processing states.
- Feed preview.
- Live sessions.
- Incidents and RCA placeholders.

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

## Role In Demo

Admin web connects the product flow and AIOps story: it observes video upload/processing, feed readiness, live sessions, service health, incidents, and RCA reports through the API gateway only.
