# db-provider
A containerized service that provides vanilla, throw-away database instances on demand for integration tests.

Imagine a GRPC service that you can query to get a fresh, empty, database instance whenever you need one for testing. The service is containerized so it's guaranteed to work anywhere, and you can run the service in Docker on your developer machine so it's always available in the background.

## Supported Database Types
The infrastructure in this module is flexible enough to support any type of database, but currently the only supported types are:

  * Mysql
  * (...[pull requests welcome!](#how-to-contribute))

If you need a database of another type, feel free to follow the Mysql example to set up a new container with a different database image and submit a pull request. This module was designed to support any type of database. Read [this section](#how-to-contribute) for info on how to contribute.

## Client Libraries
Since the service uses GRPC, it can support clients from just about any language. That being said, we currently have client implementations in the following languages:

  * Golang
  * (...[pull requests welcome!](#how-to-contribute))

You can find working examples under the language-specific client directories. These can be run to test that the provider service is working from your preferred client language once you have it up and running.

## Quickstart
Follow these instructions to get up and running quickly and see how/if everything works in your environment. At a high level, there are two basic steps:

1. Run the database provider service.
1. Run the example integration tests.

### Running The Service
This repo uses docker-compose to spin up the necessary containers in Docker to run the service.

```bash
# Go to the relevant container directory:
$ cd containers/mysql

# Start the containers using docker-compose (this will automatically pull them from the registry):
containers/mysql$ docker-compose up -d
```

The latest built container is hosted at https://hub.docker.com/r/karagog/mysql-db-provider, but you can build/fetch your own locally-built version by following the directions [below](#deploying-from-a-locally-built-version).

### Running the Example Tests
There are example integration tests available for you to study and use as a starting point for your own integration tests.

```bash
# Note that you can run without the tag and it will only run unit tests.
$ go test ./... -tags=integration
```

## Deploying From A Locally-Built Version
You can build the container yourself from source code and deploy that version by following these instructions. This is useful, for example, if you want to make changes to the container and test them quickly, or if you depend on a specific version of the service. For most use cases, however, you should be able to deploy the publicly provided version.

### Pushing A New Container Version
This repo uses Dockerfiles to build the applications and docker containers, so building and pushing an image is as simple as:

```bash
# Run this command from the workspace root:
$ docker build -f containers/mysql/Dockerfile . -t karagog/mysql-db-provider:latest

$ docker push karagog/mysql-db-provider:latest
```

### Pulling and Running
After pushing to the local registry you can pull the image and run it, simply by updating an environment variable with the address of your registry service (usually localhost:5000):

```bash
# Pull and run the image:
containers/mysql$ docker-compose pull
containers/mysql$ docker-compose up
```

## How to Contribute
Anyone is welcome to submit pull requests for new features and/or new database types and language-specific client libraries. They should be fairly straightforward to add if you follow the existing examples.

### Pre-Submit Testing
There are GitHub actions configured on this repository, so simply create a pull request to the master branch and watch the checks run!

## A Note on Parallelism
The database provider service initializes a pool of databases that are ready to go whenever needed, so it supports parallelism up to this limit. You can configure the size of this pool through the use of environment variables (see the code for reference), but keep in mind that it only makes sense for the pool to be as large as the number of tests you plan to run concurrently, otherwise there could be a large portion of the instance pool that is always idle. This is likely not an issue unless you are initializing a very large pool (e.g. thousands or millions of databases).

## A Note on Scalability
This service could be scaled beyond one database server instance if you put a load balancing service in front of the service containers, to ensure that requests don't always go to the same container. In that way this project could conceivably support a scalable integration test farm for continuous build systems. This is left as an exercise for the reader, but please send pull-requests if there are improvements we can make to the base infrastructure to make it easier to scale.
