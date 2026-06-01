# syntax=docker/dockerfile:1

# --- build the Vite bundle ---
FROM node:25-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# --- serve via nginx ---
FROM nginx:alpine
# gettext provides envsubst (already present in nginx:alpine, installed defensively).
RUN apk add --no-cache gettext
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf.template /etc/nginx/nginx.conf.template
COPY docker-entrypoint.sh /docker-entrypoint-railway.sh
RUN chmod +x /docker-entrypoint-railway.sh
# Remove the stock default server so only our rendered config is loaded.
RUN rm -f /etc/nginx/conf.d/default.conf
ENTRYPOINT ["/docker-entrypoint-railway.sh"]
