FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/platform-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/platform-api /platform-api
EXPOSE 8080
ENTRYPOINT ["/platform-api"]
