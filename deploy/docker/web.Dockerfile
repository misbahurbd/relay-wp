# Build context: repository root.
FROM node:24.22.1-alpine AS build
RUN corepack enable
WORKDIR /src
COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY apps/web/package.json apps/web/
COPY packages/api-client/package.json packages/api-client/
RUN pnpm install --frozen-lockfile
COPY apps/web/ apps/web/
COPY packages/ packages/
RUN pnpm --filter @relay/web build

# Nitro's .output is self-contained, so the runtime image needs no node_modules.
FROM node:24.22.1-alpine
WORKDIR /app
ENV NODE_ENV=production PORT=3000
COPY --from=build /src/apps/web/.output ./.output
USER node
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
