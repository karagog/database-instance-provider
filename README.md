# db-provider
A containerized service that provides vanilla, throw-away database instances on demand for integration tests.

Imagine a GRPC service that you can query to get a fresh, empty, database instance whenever you need one for testing. The service is containerized so it's guaranteed to work anywhere, and you can run the service in Docker on your developer machine so it's always available in the background.

## Supported Database Types
The infrastructure in this module is flexible enough to support any type of database, but currently the only supported types are:

  * Mysql
  * (...pull requests welcome!)

If you need a database of another type, feel free to follow the Mysql example to set up a new container with a different database image and submit a pull request. This module was designed to support any type of database.

## Client Libraries
Since the service uses GRPC, it can support clients from just about any language. That being said, we have client implementations in the following languages:

  * Golang
  * (...pull requests welcome!)

### Golang Client Example

```go
import (
  "github.com/karagog/db-provider/client/go/database"
  "github.com/karagog/db-provider/client/go/database/mysql"
)

func TestFoo(t *testing.T) {
  // This call blocks until a database is available.
  // If anything goes wrong such that we can't get a database
  // instance, this panics to abort the test.
  instance := database.NewFromFlags(context.Background())

  // Always close the instance when done, to release the
  // database so another test can use it.
  defer instance.Close()

  // Connect to the database using either the root connection
  // (with full privileges) or a less privileged connection for
  // use by application code.
  dbRoot := mysql.ConnectOrDie(instance.Info.RootConn)
  dbApp := mysql.ConnectOrDie(instance.Info.AppConn)

  // After that, you can freely use (or abuse) the database
  // with whatever tests you need to do. After you're done
  // the database will be completely reset so there's no fear
  // of leaving it in a weird state for the next test.
}
```

## Deploying The Service
The latest built container is currently hosted at https://hub.docker.com/r/karagog/mysql-db-provider, so the simplest way to deploy it is to just pull it from there like this:

```bash
#!/bin/bash

# These values must match the values used by the service.
# They can be changed by passing new env values.
PROVIDER_PORT=58615
MYSQL_PORT=53983

# This runs the docker container so it's always up.
# Notice we use tmpfs for the mysql directory so it's fast.
docker run -d --name mysql_provider --restart always \
  -p $PROVIDER_PORT:$PROVIDER_PORT \
  -p $MYSQL_PORT:3306 \
  --tmpfs=/var/lib/mysql \
  karagog/mysql-db-provider:latest
```

## Deploying From A Locally-Built Version
You can build the container yourself from source code and deploy that version by following these instructions. This is useful, for example, if you want to make changes to the container and test them quickly, or if you depend on a specific version of the service. For most use cases, however, you should be able to deploy the publicly provided version.

### Pushing A New Container Version
This repo uses Bazel rules to build the applications and docker containers, so building and pushing an image is as simple as:

```bash
# This pushes to a local Docker registry
# (you must have one running).
$ bazel run //containers/mysql:push_local
```

### Pulling and Running
After pushing you can pull the image and run it, simply by prefixing the deployment command above with the address of your registry service (usually localhost:5000), or by running docker-compose in the container's source code directory.

```bash
# First go to the container directory, where there is a
# config file called docker-compose.yml.
$ cd containers/mysql

# Pull the image from the repo in the config.
$ docker-compose pull

# Run the image according to the parameters in docker-compose.yml.
$ docker-compose up
```

## A Note on Parallelism
The database provider service initializes a pool of databases that are ready to go whenever needed, so it supports parallelism up to this limit. You can configure the size of this pool through the use of environment variables (see the code for reference), but keep in mind that it only makes sense for the pool to be as large as the number of tests you plan to run concurrently, otherwise there could be a large portion of the instance pool that is always idle. This is likely not an issue unless you are initializing a very large pool (e.g. thousands or millions of databases).

## A Note on Scalability
This service could be scaled beyond one container instance if you put a load balancing service in front of them, to ensure that requests don't always go to the same container. In that way this project could conceivably support a scalable integration test farm for continuous build systems. This is left as an exercise for the reader, but please send pull-requests if there are improvements we can make to the base infrastructure to make it easier to scale.
