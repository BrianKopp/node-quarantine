# Node Quarantine

This project is designed to help with the following situation.

You have a set of cluster nodes that run ephemeral workloads that
you don't want to disrupt with operations like node drain.
Normally, you scale your nodes up with cluster autoscaler,
but you don't want a cluster autoscaler to scale the nodes down
if there are any of those workloads running.

To accomplish this, you implement PodDisruptionBudgets on your
workloads for whom disruptions are undesirable. You want to still be
able to scale up as necessary using Cluster Autoscaler, over-provisioners,
etc. etc., but are OK with much slower node scale-down cycles, as a
tradeoff to help your workloads stay uninterrupted.

## How it Works

This application monitors (some of) your nodes, and waits for them to become
sufficiently under-utilized for a period of time. Once that occurs,
a node is selected to be cordoned, not drained. By cordoning the node,
its ephemeral workloads are allowed time to complete and exit, but no
new workloads are allowed to be scheduled. Once your workloads have
completed, and your node is cordoned, there is nothing stopping Cluster Autoscaler
from completing the drain and termination of your node.

## An Example Application

You run medium-duration Jobs that perform some important function.
They are feeding off of real-time data and take minutes to complete.
You'd like their results as quickly as possible, and as such, you would
prefer that they not be restarted unless absolutely necessary (e.g.
some kind of node failure or spot termination). You want your nodes to
scale up with demand, but don't want Cluster Autoscaler to decide a node
is underutilized and evict your pod to enable a node termination.
Thus you include a PodDisruptionBudget to prevent Cluster Autoscaler
disruptions, but need a way for the fleet of nodes to scale down to
maintain some degree of resource efficiency.

That's where this application comes in!

