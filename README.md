[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/compass-manager)](https://api.reuse.software/info/github.com/kyma-project/compass-manager)

# Compass Manager

## Overview
Compass Manager **will be** a new Control Plane component. Build using [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework. 

It's main responsibilities **will be**:
- registering Kyma runtimes in the Compass Director
- creating secret on client cluster with the Compass Runtime configuration

## Prerequisites

- Golang - minimum version is 1.20.
- Kyma Custom Resource Definition present on cluster.
- Access to a k8s cluster.
- `kcp-system` namespace on the k8s cluster
- [k3d](https://k3d.io) to get a local cluster for testing, or run against a remote cluster.
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [kubebuilder](https://book.kubebuilder.io/)


1. Clone the project.

```bash
git clone https://github.com/kyma-project/compass-manager.git && cd compass-manager/
```

2. Set the `compass-manager` image name. Add prefix `your-docker-hub-user/` if needed

```bash
export IMG=custom-compass-manager:local
```

3. Build the project.

```bash
make build
```

4. Build the image.

```bash
make docker-build
```

5. Push the image to the external DockerHub registry. 

```bash
make docker-push
```

6. Create a Secret containing Compass Director authorization data:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kcp-provisioner-credentials-file
  namespace: kcp-system
type: Opaque
stringData:
  director.yaml: |-
    data:
      client_id: "some-ID"
      client_secret: "some-Secret"
      tokens_endpoint: "https://example.com/oauth2/token"
```

7. Deploy.

```bash
make deploy
```
## Usage

Compass Manager watches for Kyma Custom Resource changes, and when a Kyma with `application-connector` module is created, it registers the Kyma runtime in the Compass Director and creates a Compass Manager Mapping with the ID assigned by the Compass Director.
It then configures the Compass Runtime Secret on the client cluster.

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: Kyma
metadata:
  name: 54572f7a-b2c2-4f09-b83e-1c9f9b690e02
  namespace: kcp-system
  labels:
    kyma-project.io/broker-plan-name: open
    kyma-project.io/broker-plan-id: 925c16be-3644-457b-840b-768f69b993cf
    kyma-project.io/instance-id: 1ccf9b07-1296-4052-97b5-1c38aa72ac41
    kyma-project.io/shoot-name: my-shoot
    kyma-project.io/subaccount-id: 170ba3ca-6905-466a-a109-f2a6efdca439
    kyma-project.io/global-account-id: b07fb88f-a100-4471-bb71-8adb400a3f7f
    operator.kyma-project.io/kyma-name: 54572f7a-b2c2-4f09-b83e-1c9f9b690e02
spec:
  channel: regular
status:
  modules:
    - name: "some-module"
      state: "Ready"
    - name: "application-connector"
      state: "Ready"
```

### Configuration Envs

| Name                               | Default                                                                      | Description                                                                         |
|------------------------------------|------------------------------------------------------------------------------|-------------------------------------------------------------------------------------|
| `APP_ADDRESS`                      | `127.0.0.1:3000`                                                             | Address on which the app is exposed                                                 |
| `APP_APIENDPOINT`                  | `/graphql`                                                                   | Endpoint for GraphQL requests                                                       |
| `APP_SKIPDIRECTORCERTVERIFICATION` | `false`                                                                      | Skips cert verification in the Compass Director GraphQL calls                       |
| `APP_DIRECTOR_URL`                 | `https://compass-gateway-auth-oauth.mps.dev.kyma.cloud.sap/director/graphql` | URL of the Compass Director GraphQL endpoint                                        |
| `APP_DIRECTOR_OAUTH_PATH`          | `./dev/director.yaml`                                                        | File with OAuth data for Compass Director                                           |
| `APP_ENABLED_REGISTRATION`         | `false`                                                                      | Enable registering runtimes with Compass                                            |
| `APP_DRYRUN`                       | `false`                                                                      | Disable registering and configuring; instead log which operations would be executed |

> **TIP:** `CompassManagerMappings` created with dry run are labeled `kyma-project.io/cm-dry-run: Yes`

## Development

To build the project, use the following command:
```shell
make build
````

To run the project locally:
```shell
./bin/manager -kubeconfig <PATH TO KUBECONFIG>
```

To run the tests:
```shell
make test
```

Controller is tested with the use of the [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) package.
You can run single envtest with the following command:
```shell
make manifests generate fmt vet envtest
KUBEBUILDER_ASSETS="$PWD/$(./bin/setup-envtest use 1.26.0 --bin-dir ./bin -p path)" go test ./controllers -v -run TestAPIs -ginkgo.focus "Kyma was already registered, but doesn't have a Compass Mapping"
```