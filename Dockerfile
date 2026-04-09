FROM node:25-alpine AS assets
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install
COPY assets ./assets
COPY internal/http/templates ./internal/http/templates
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
COPY --from=assets /app/public/assets ./public/assets
RUN go build -o /out/app ./cmd/app

FROM alpine:3.22
WORKDIR /app
RUN adduser -D -g '' appuser
COPY --from=build /out/app /app/app
COPY --from=build /src/public/assets /app/public/assets
COPY --from=build /src/db /app/db
COPY --from=build /src/seed /app/seed
COPY --from=build /src/internal/http/templates /app/internal/http/templates
USER appuser
EXPOSE 8080
CMD ["/app/app", "web"]
