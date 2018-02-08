# Distcc
Distcc servers are a shared resource. Users can use the same set of servers to
compile their native code. We want to scale the numer of servers up and down
depending on the usage.

Distcc has the following assumptions:
* The client side expects a static list of servers in the environment to be
  provided before the compilation starts.
* When a source file needs to be compiled, the client selects a server and tries
  to compile there. If the server is not reachable, after a few retries, the
  compilation is done locally.

To allow downscaling, we need to "pack" the users in a reasonable amount of
servers, depending on the load, so we can tear down some instances when the
number of users decrease.

To avoid degrading the service for users that are compiling at the moment of a
downscale, we assign servers to users and pair this allocation with an
expiration time. If a server is allocated to at least a user, it can't be
downscaled.

To facilitate downscales, we define a maximum number of clients assigned to
every host. The hosts are part of a stateful set, so they are ordered. Users are
assigned to the hosts with the minimal ordinal, unless they are full. In this
way we "pack" users together and we are able to terminate the idle servers with
higher ordinals.

# Icecc
Icecc seems to have an interesting feature, that is load balancing across
the servers, depending on the load they are experiencing. The downsides are that
the project seems to be stale
