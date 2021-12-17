FROM golang:1.17-alpine

# You can optionally use the non-alpine image for easier debugging,
# since it has more utilities like a shell.
# FROM golang:1.17

LABEL maintainer George Karagoulis (karagog@gmail.com)

# Copy the source files to the container and build them.
WORKDIR /src/mysql-db-provider
COPY . ./
RUN go build -o /mysql-db-provider ./containers/mysql/provider
RUN go build -o /healthcheck ./healthcheck
RUN go build -o /healthcheck ./healthcheck

# Change the workdir after building successfully, because we'll throw away the source.
WORKDIR /

# Remove the source code and Go modules after building, to reduce space.
RUN rm /src "$GOPATH/pkg/*" -rf

# Add the health check so Docker will know when the service is healthy.
# HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
#   CMD "/healthcheck"

# Run the database provider executable.
CMD [ "/mysql-db-provider" ]
