FROM node:22-alpine AS builder

WORKDIR /workspace

COPY apps/client/package.json apps/client/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile

COPY apps/client ./

ARG VITE_API_BASE_URL=/
ARG VITE_CLIENT_MODE=single-server-web
ARG VITE_SINGLE_SERVER_BASE_URL=/

ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
ENV VITE_CLIENT_MODE=${VITE_CLIENT_MODE}
ENV VITE_SINGLE_SERVER_BASE_URL=${VITE_SINGLE_SERVER_BASE_URL}

RUN pnpm build

FROM nginx:1.27-alpine

COPY deploy/nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=builder /workspace/build /usr/share/nginx/html
