# syntax=docker/dockerfile:1.7

FROM node:22-alpine3.23 AS deps

WORKDIR /app
RUN apk add --no-cache libc6-compat
COPY package.json package-lock.json .npmrc ./
COPY web/package.json ./web/package.json
RUN npm ci

FROM node:22-alpine3.23 AS build

WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=deps /app/node_modules ./node_modules
COPY --from=deps /app/web/node_modules ./web/node_modules
COPY --from=deps /app/package.json ./package.json
COPY --from=deps /app/package-lock.json ./package-lock.json
COPY --from=deps /app/web/package.json ./web/package.json
COPY web ./web
RUN npm --workspace web run build

FROM node:22-alpine3.23 AS dev

WORKDIR /app
ENV NODE_ENV=development
ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0
ENV PORT=3000

COPY --from=deps /app/node_modules ./node_modules
COPY --from=deps /app/web/node_modules ./web/node_modules
COPY --from=deps /app/package.json ./package.json
COPY --from=deps /app/package-lock.json ./package-lock.json
COPY --from=deps /app/.npmrc ./.npmrc
COPY web ./web

EXPOSE 3000
CMD ["npm", "--workspace", "web", "run", "dev", "--", "--hostname", "0.0.0.0", "--port", "3000"]

FROM node:22-alpine3.23 AS runner

LABEL org.opencontainers.image.source="https://github.com/sbekti/intern"

WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0
ENV PORT=3000

COPY --from=build /app/web/.next/standalone ./
COPY --from=build /app/web/public ./web/public
COPY --from=build /app/web/.next/static ./web/.next/static

EXPOSE 3000
CMD ["node", "web/server.js"]
