# Build context: repository root.
FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/relay ./cmd/relay

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/relay /relay
ENV GOMEMLIMIT=800MiB
EXPOSE 8080
ENTRYPOINT ["/relay"]
CMD ["api"]
