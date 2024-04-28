# syntax=docker/dockerfile:1

FROM golang:1.22-alpine3.18 AS BUILD

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY * /app/

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /polinema-api

FROM alpine:3.19

COPY --from=BUILD /polinema-api /app/polinema-api

COPY --from=BUILD /app/*.html /app/

COPY --from=BUILD /app/*.json /app/

# Optional:
# To bind to a TCP port, runtime parameters must be supplied to the docker command.
# But we can document in the Dockerfile what ports
# the application is going to listen on by default.
# https://docs.docker.com/reference/dockerfile/#expose
EXPOSE 8080

# Run
CMD ["/app/polinema-api"]