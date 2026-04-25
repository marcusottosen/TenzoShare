FROM node:22-alpine AS builder
WORKDIR /app
COPY web/admin-portal/package*.json ./
RUN npm ci --ignore-scripts
COPY web/admin-portal/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY web/nginx-spa.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist/ /usr/share/nginx/html/
EXPOSE 80
