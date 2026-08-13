# Intern web app

This Next.js app is the authenticated web interface for Intern. It uses the
Base UI + Vega shadcn setup and is developed from the repository root:

```sh
./scripts/dev.sh
```

The root Compose watch stack provides the private API service. The web service
is the only published service and binds to `0.0.0.0:3000` by default. See the
root [`README.md`](../README.md) for production-service warnings, configuration,
and development identity options.

Use the shadcn CLI to add components only after checking the installed Base UI
registry and reviewing the dry-run diff:

```sh
npx shadcn@latest info
npx shadcn@latest docs <component>
```
