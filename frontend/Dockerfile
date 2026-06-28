# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM node:20-alpine AS build
WORKDIR /app

# Install dependencies first (better layer caching)
COPY package.json package-lock.json ./
RUN npm ci

# Build the app
COPY . .
RUN npm run build

# ---- Serve stage ----
FROM nginx:alpine AS runtime

# SPA-aware nginx config (fallback to index.html, proxy API to backend)
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Static assets produced by the build stage
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
