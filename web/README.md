# Intern web app

This Next.js app is the authenticated web interface for Intern. It uses the
Base UI + Vega shadcn setup and is developed from the repository root:

```sh
npm ci
npm run dev
```

The root Compose watch stack provides the private API service. The web service
is the only published service and binds to loopback.

Use the shadcn CLI to add components only after checking the installed Base UI
registry and reviewing the dry-run diff:

```sh
npx shadcn@latest info
npx shadcn@latest docs <component>
```
