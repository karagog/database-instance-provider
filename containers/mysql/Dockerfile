# This builds a Docker image for the provider service.
#
# Example (run in source root):
#
#   $ docker build -f containers/mysql/Dockerfile . -t karagog/mysql-db-provider:latest

# Use a really small image, which has the minimal environment necessary to build
# and run Go programs.
FROM golang:1.17-alpine

# You can optionally use the non-alpine image for easier debugging,
# since it has all necessary utilities (like a shell, and apt-get).
# FROM golang:1.17

LABEL maintainer George Karagoulis <karagog@gmail.com>

# Copy the source files to the container.
WORKDIR /usr/local/src/db-provider
COPY . .

# Build the binaries into the output directory.
RUN go build -o /usr/local/bin ./containers/mysql/provider

# Add the health check so Docker will know when the service is healthy.
# This polls on a tight interval to minimize the test setup time by marking
# it healthy as soon as possible.
HEALTHCHECK --interval=1s --timeout=5s --retries=3 \
  CMD RESP="$(wget http://localhost/healthcheck -O /dev/stdout)"; if [ "$RESP" != "OK" ]; then exit 1; fi;

# Run the database provider executable.
ENTRYPOINT [ "provider" ]
