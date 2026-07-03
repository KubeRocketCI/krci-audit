# Single image for both deployables: the migration Job (krci-audit-migrate) and the read API
# (krci-audit-api). The platform build pipeline publishes exactly one image per codebase, so
# both binaries ship together; the Job/Deployment select which one runs via `command`.
# Use distroless as minimal base image to package the binaries
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
WORKDIR /
COPY ./dist/krci-audit-migrate-${TARGETARCH} /krci-audit-migrate
COPY ./dist/krci-audit-api-${TARGETARCH} /krci-audit-api

USER 65532:65532

ENTRYPOINT ["/krci-audit-migrate"]
