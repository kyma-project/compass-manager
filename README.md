[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/compass-manager)](https://api.reuse.software/info/github.com/kyma-project/compass-manager)

# Compass Manager

## Overview
Compass Manager **will be** a new Control Plane component. Build using [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework. 

It's main responsibilities **will be**:
- registering Kyma runtimes in the Compass Director
- creating secret on client cluster with the Compass Runtime configuration

## Prerequisites

- Golang - minimum version is 1.20.
- Access to a k8s cluster.
- Kyma Custom Resource Definition is present on cluster.
- [k3d](https://k3d.io) to get a local cluster for testing, or run against a remote cluster.
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [kubebuilder](https://book.kubebuilder.io/)

## Installation

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

6. Create a YAML file containing Compass Director authorization data and encode entire YAML in Base64 format:

```yaml
data:
  client_id: some-id
  client_secret: some-secret
  tokens_endpoint: some-endpoint
```

7. Provide previously encoded file in `director.yaml` field and apply the secret on cluster where you want to run Compass Manager

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kcp-provisioner-credentials-file
  namespace: kcp-system
type: Opaque
data:
  director.yaml: base64-encoded-yaml
```

8. Deploy.

```bash
make deploy
```
## Usage

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

> Explain how to use the project. You can create multiple subsections (H3). Include the instructions or provide links to the related documentation.

## Development

> Add instructions on how to develop the project or example. It must be clear what to do and, for example, how to trigger the tests so that other contributors know how to make their pull requests acceptable. Include the instructions or provide links to related documentation.

## Troubleshooting

> List potential issues and provide tips on how to avoid or solve them. To structure the content, use the following sections:
>
> - **Symptom**
> - **Cause**
> - **Remedy**
