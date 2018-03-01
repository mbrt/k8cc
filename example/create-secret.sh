#!/bin/bash
set -e

PUBKEY=${HOME}/.ssh/id_*.pub
kubectl create secret generic mike-ssh-key --from-file=authorized_keys=${PUBKEY}
