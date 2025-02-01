# kubectl-colorize-managed-fields

A kubectl plugin to display colorized fields of the kubernetes resources based on their managed fields.

## Installation

### Using go install
```console
$ go install github.com/tt-kuma/kubectl-colorize-managed-fields/cmd/kubectl-colorize_managed_fields@latest
```

## Usage

Here is an example of a resource applied by specifying a field manager.
```shell
$ cat <<EOF | kubectl apply -f - --server-side --field-manager sample-manager
apiVersion: v1
kind: ConfigMap
metadata:
  name: sample
data:
  key1: value1
  key2: value2
  key3: value3
EOF
```
Run `kubectl colorize-managed-fields`. You can specify target resources like `kubectl get`.
```shell
$ kubectl colorize-managed-fields configmaps sample
```
You can see the following yaml with colorized fields based on managed fields.
![](https://github.com/tt-kuma/kubectl-colorize-managed-fields/blob/image/images/example-single-manager.png)


For another example, overwrite some fields by specifying another field manager.
```shell
$ cat <<EOF | kubectl apply -f - --server-side --field-manager new-sample-manager --force-conflicts
apiVersion: v1
kind: ConfigMap
metadata:
  name: sample
data:
  key1: new-value1
  key2: value2
EOF
```
When multiple manager exists, each manager assigned a different color. The fields managed by multiple managers are colorized by fixed color indicating a conflict.
![](https://github.com/tt-kuma/kubectl-colorize-managed-fields/blob/image/images/example-multiple-managers.png)

## License

kubectl-colorize-managed-fields is licensed under the Apache License 2.0.
