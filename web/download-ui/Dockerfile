FROM node:22-alpine AS builder
WORKDIR /app
COPY web/download-ui/package*.json ./
RUN npm ci --ignore-scripts
COPY web/download-ui/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY web/nginx.conf /etc/nginx/nginx.conf
COPY web/nginx-spa.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist/ /usr/share/nginx/html/
EXPOSE 80
