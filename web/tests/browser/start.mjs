import { cpSync } from "node:fs"
cpSync(".next/static", ".next/standalone/web/.next/static", { recursive: true })
await import("../../.next/standalone/web/server.js")
