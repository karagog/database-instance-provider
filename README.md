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

## Running The Service
This repo uses docker-compose to spin up the necessary containers in Docker to run the service. Simply "cd" into the relevant container directory (e.g. containers/mysql) and run:

```bash
# First fetch the images from their respective registries.
containers/mysql$ docker-compose pull

# Then start the containers using docker-compose:
containers/mysql$ docker-compose up -d
```

The latest built container is hosted at https://hub.docker.com/r/karagog/mysql-db-provider, but you can build/fetch your own locally-built version by following the directions below.

## Deploying From A Locally-Built Version
You can build the container yourself from source code and deploy that version by following these instructions. This is useful, for example, if you want to make changes to the container and test them quickly, or if you depend on a specific version of the service. For most use cases, however, you should be able to deploy the publicly provided version.

### Pushing A New Container Version
This repo uses Bazel rules to build the applications and docker containers, so building and pushing an image is as simple as:

```bash
# Make sure you have a local registry service running
# (you only need to do this once):
$ docker run -d -p 5000:5000 --restart=always --name registry registry:2

# This pushes to the local Docker registry:
$ bazel run //containers/mysql:push_local
```

### Pulling and Running
After pushing you can pull the image and run it, simply by prefixing the deployment command above with the address of your registry service (usually localhost:5000), or by running docker-compose in the container's source code directory.

```bash
# Update the environment to fetch from the local registry:
containers/mysql$ export DOCKER_REGISTRY_ADDRESS=localhost:5000

# Pull and run the local image:
containers/mysql$ docker-compose pull
containers/mysql$ docker-compose up
```

## How to Contribute
Anyone is welcome to submit pull requests for new features and/or new database types and language-specific client libraries. They should be fairly straightforward to add if you follow the existing examples.

### Pre-Submit Testing
Run `scripts/presubmit.sh` to run all tests before submitting a pull request. That script is the source of truth for any necessary validations.

```bash
$ ./scripts/presubmit.sh
```

You must ensure that the exit code was 0, and if any files were changed that you commit those changes as well (e.g. updated BUILD, or auto-formatted files).

## A Note on Parallelism
The database provider service initializes a pool of databases that are ready to go whenever needed, so it supports parallelism up to this limit. You can configure the size of this pool through the use of environment variables (see the code for reference), but keep in mind that it only makes sense for the pool to be as large as the number of tests you plan to run concurrently, otherwise there could be a large portion of the instance pool that is always idle. This is likely not an issue unless you are initializing a very large pool (e.g. thousands or millions of databases).

## A Note on Scalability
This service could be scaled beyond one database server instance if you put a load balancing service in front of the service containers, to ensure that requests don't always go to the same container. In that way this project could conceivably support a scalable integration test farm for continuous build systems. This is left as an exercise for the reader, but please send pull-requests if there are improvements we can make to the base infrastructure to make it easier to scale.
