# syntax=docker/dockerfile:1

FROM node:22-alpine3.24 AS deps

WORKDIR /app
RUN apk add --no-cache libc6-compat
COPY package.json package-lock.json ./
COPY web/package.json ./web/package.json
RUN npm ci

FROM deps AS build

ENV NEXT_TELEMETRY_DISABLED=1
COPY web ./web
RUN npm --workspace web run build

FROM deps AS dev

ENV NODE_ENV=development
ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0
ENV PORT=3000

COPY web ./web

EXPOSE 3000
CMD ["npm", "--workspace", "web", "run", "dev", "--", "--hostname", "0.0.0.0", "--port", "3000"]

FROM node:22-alpine3.24 AS runner

LABEL org.opencontainers.image.source="https://github.com/sbekti/intern"

WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0
ENV PORT=3000

COPY --from=build /app/web/.next/standalone ./
COPY --from=build /app/web/.next/static ./web/.next/static

EXPOSE 3000
CMD ["node", "web/server.js"]
