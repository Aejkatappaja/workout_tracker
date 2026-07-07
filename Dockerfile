# build a static, self-contained binary (assets, migrations and templ output are embedded)
FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /go-gym .

# minimal runtime: distroless static, non-root
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /go-gym /go-gym
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/go-gym"]
