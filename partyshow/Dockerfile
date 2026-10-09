FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /partyshow ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /partyshow /partyshow
ENV ADDR=:8080 DATA_DIR=/data
VOLUME /data
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/partyshow"]
