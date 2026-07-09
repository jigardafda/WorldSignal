# Build the WorldSignal Go backend.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/server /server
EXPOSE 4800
ENV HOST=0.0.0.0 PORT=4800 ROLE=all
ENTRYPOINT ["/server"]
