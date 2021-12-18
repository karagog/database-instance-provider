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
ARG workdir=/opt/mysql-db-provider
WORKDIR ${workdir}
COPY . ./

# Build the binaries into the output directory.
ARG output_dir=bin
RUN mkdir ${output_dir}

RUN go build -o ${output_dir} ./containers/mysql/provider ./healthcheck

ENV PATH="${workdir}/${output_dir}:${PATH}"

# Add the health check so Docker will know when the service is healthy.
# This polls on a tight interval to minimize the test setup time by marking
# it healthy as soon as possible.
HEALTHCHECK --interval=1s --timeout=5s --retries=3 \
  CMD "healthcheck"

# Run the database provider executable.
CMD [ "provider" ]
