# syntax=docker/dockerfile:1.4

FROM golang:1.18 AS builder
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

ARG TARGETARCH  # 定义 ARG
RUN echo "Building for architecture: $TARGETARCH"

RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o myapp .

FROM scratch
COPY --from=builder /app/myapp .
EXPOSE 9800
CMD ["./myapp"]

