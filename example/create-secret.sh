#!bin/bash
kubectl create secret generic mike-ssh-key --from-file=authorized_keys=${HOME}/.ssh/id_rsa.pub
