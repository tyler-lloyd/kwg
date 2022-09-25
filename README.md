# kwg

[![Go Report Card](https://goreportcard.com/badge/github.com/tyler-lloyd/kwg)](https://goreportcard.com/report/github.com/tyler-lloyd/kwg)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/tyler-lloyd/kwg)

Kubernetes WireGuard server EZ mode.

## About

[kwg](https://marvelcinematicuniverse.fandom.com/wiki/Korg) is a utility for deploying and configuring a WireGuard server inside a Kubernetes cluster. The wg server will act as a subnet router to the rest of the cluster so peers can have direct access to pods and services without exposing them publicly.

It can be used as an alternative to `kubectl port-forward` or just as a way to access k8s pods through an encrypted tunnel.

## Usage

Build from source

```
git clone https://github.com/tyler-lloyd/kwg
cd kwg
make
```

Then deploy the server

```sh
./kwg deploy --kubeconfig /path/to/kubeconfig
```

Then add this machine as a peer and route all pod and service CIDR traffic through the tunnel

```sh
# example: the cluster pod CIDR is 10.244.0.0/16 and the server CIDR is 10.0.0.0/16

./kwg join --allowed-ips 10.244.0.0/16,10.0.0.0/16 --kubeconfig /path/to/kubeconfig
```

Now you should be able to access any pod or service from their cluster IP directly without any port-forwarding.
