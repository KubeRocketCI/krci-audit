# Migrator image: applies the embedded schema migrations. Run as a Helm install/upgrade Job
# with AUDIT_DB_DSN (or PG* env) from a Secret.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/krci-audit-migrate ./cmd/krci-audit-migrate

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/krci-audit-migrate /krci-audit-migrate
USER nonroot:nonroot
ENTRYPOINT ["/krci-audit-migrate"]
