FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 go build -o /bin/worker ./cmd/worker

FROM alpine:3.21 AS server
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/server /usr/local/bin/server
ENTRYPOINT ["server"]

FROM alpine:3.21 AS worker
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/worker /usr/local/bin/worker
ENTRYPOINT ["worker"]
