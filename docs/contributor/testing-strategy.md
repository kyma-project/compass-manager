# Testing Strategy for Compass Manager

## Introduction
This testing strategy describes how the Framefrog team tests the Kyma Compass Manager product. It outlines the approach and methodologies used for testing all layers of this product to ensure the following:

* Stability by applying load and performance tests that verify the product resilience under load peaks.
* Reliability by running chaos tests and operational awareness workshops to ensure product reliability in a productive context.
* Functional correctness by documenting and tracing all product features, their acceptance criteria and the used end-2-end tests to verify a correct implementation.
* Maintainability by regularly measuring code quality metrics like reusability, modularity, and package qualities.


## Testing Methodology

We investigate the product by dividing it into layers:

* Code: Includes the technical frameworks (e.g. Kubebuilder) and custom Golang code.
* Business Features: Combines the code into a feature our customers consume.
* Product Integration: Verifies how our product is integrated into the technical landscape, how it interacts with 3rd party systems, and how it is accessible by customers or remote systems.
 
For each layer, a dedicated testing approach is used:

* **Unit Testing for Code:** Writing and executing tests for individual functions, methods, and components to verify their behavior and correctness in isolation.
* **Integration Testing for Business Features:** Validating the integration and interaction between different components, modules, and services in the project.
* **End-to-End Testing:** Testing the application as a whole in a production-like environment, mimicking real-world scenarios to ensure the entire system is secure, functions correctly, and performs well.

## Testing Approach

### Unit Testing

This section focuses on ensuring the functionality and reliability of critical functions, methods, and components within our system.

1. Identify critical functions, methods, and components that require testing.
2. Write unit tests using GoUnit tests, Ginkgo, and Gomega frameworks.
3. Ensure tests cover various scenarios, edge cases, and possible failure scenarios. We try to verify business relevant logic with at least 65% code coverage.
4. Test for both positive and negative inputs to validate the expected behavior.
5. Mock external dependencies and use stubs or fakes to isolate the unit under test.
6. Run unit tests periodically during development and before each PR is merged to prevent regressions.
7. Unit tests must be executed as fast as possible to minimize roundtrip times. Long-running tests should be excluded from frequently executed test runs and be triggered periodically, for example, 4 times a day.

### Integration Testing

This section focuses on the integration testing process, which involves testing the interaction and integration of various components and custom resources with the Kubernetes API. It provides you with a step-by-step guide to conduct integration testing, ensuring the correctness and functionality of the implemented business features.

1. The PO and the team create a registry of implemented business features and define a suitable test scenario for each feature.
2. Create a separate test suite for integration testing.
3. Each test scenario is implemented in a separate test case. Use the Kubebuilder Test Framework and others to create test cases that interact with the Kubernetes cluster.  
4. Test the interaction and integration of your custom resources, controllers, and other components with the Kubernetes API.
5. Ensure test cases cover various aspects such as resource creation, updating, deletion, and handling of edge cases.
6. Validate the correctness of event handling, reconciliation, and other control logic.
7. Integration tests must be executed fast to minimize roundtrip times and be applied for each PR. Long-running tests should be excluded from frequently executed test runs and be triggered periodically, for example, 4 times a day.

### End-to-End Testing

This section describes how to create and manage test clusters using mainstream Kubernetes management tools like Helm or Kustomize, and how to perform regular performance tests to ensure your application functions correctly and meets the KPIs in a production-like environment.

1. Use a mainstream Kubernetes management tool (for example, [Helm](https://helm.sh/) or [Kustomize](https://kustomize.io/)) to create, deploy, and manage test clusters and environments that closely resemble the productive execution context.
2. For short-living Kubernetes clusters, use k3d or other lightweight Kubernetes cluster providers.
3. Run regularly, but at least once per release, a performance test that measures product KPIs to indicate KPI violations or performance differences between release candidates.

|Testing Approach|Per Commit|Per PR|Per Release|In intervals|
|--|--|--|--|--|
|Unit Testing|X|X||Only long-running tests daily|
|Integration Testing||X||Only long-running tests daily|
|End-to-End Testing|||X|Daily|

### Testing Tools and Frameworks
Use the following tools and frameworks to implement the above-mentioned testing levels:

- **[golanci-lint](https://github.com/golangci/golangci-lint)**: Golang code linting for better code quality.
- **[go-critic](https://github.com/go-critic/go-critic)**: Another linter for measuring different code quality metrics.
- **[go test](https://pkg.go.dev/testing)**: For unit testing of Golang code.
- **Kubebuilder Test Framework and [EnvTest](https://book.kubebuilder.io/reference/envtest.html)**: For creating and executing integration tests that interact with the Kubernetes API.
- **[Ginkgo](https://github.com/onsi/ginkgo) and [Gomega](https://github.com/onsi/gomega)**: For writing and executing unit tests with a BDD-style syntax and assertions.
- **[k3d](https://k3d.io/)**: For creating short-living and lightweight Kubernetes clusters running within a Docker context.
- **[Helm](https://helm.sh/)**: For deploying and managing test clusters and environments for end-to-end testing.
- **[k6](https://k6.io/)**: For performance and stress testing.

|Framework|Unit Testing|Integration Testing|End-to-End Testing|
|--|--|--|--|
|Golangci-lint| X | | |
|Go-critic| X | | |
|go test| X |  |  |
|Kubebuilder Test Framework| X | X | |
|EnvTest| X | X |  |
|Ginkgo| X | X |  |
|Gomega| X | X |  |
|k3d|  |  | X |
|Helm|  |  | X |
|k6|  |  | X |


## Test Automation

The following CI/CD jobs are a part of the development cycle and execute quality assurance-related steps:

> **NOTE:** Jobs marked with `pull_request` are triggered with each pull request (PR). Jobs marked with `push` are executed after the merge.

- `documentation / markdown-link-check (pull_request)` - Checks if there are no broken links in the pull request `.md` files. For the configuration details, see [`mlc.config.json`](https://github.com/kyma-project/compass-manager/blob/main/mlc.config.json).
- `security-checks / govuln (pull_request)` and `Run vuln check / test (push)` - Runs [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) on the code to detect known vulnerabilities. For the configuration details, see [`run-vuln-check.yaml`](https://github.com/kyma-project/compass-manager/blob/main/.github/workflows/run-vuln-check.yaml).
- `security-checks / trivy (pull_request)` - Runs the [trivy](https://trivy.dev/) scanner on the code to detect known vulnerabilities. For the configuration details, see [`trivy.yml`](https://github.com/kyma-project/compass-manager/blob/main/.github/workflows/trivy.yml).
- `tests / unit-tests (pull_request)` - Executes basic create/update/delete functional tests of the reconciliation logic. For the configuration details, see [`tests.yml`](https://github.com/kyma-project/compass-manager/blob/main/.github/workflows/tests.yml).
- `utils / golangci-lint (pull_request)` - Is responsible for linting and static code analysis. For the configuration details, see [golangci.yaml](https://github.com/kyma-project/compass-manager/blob/main/.golangci.yaml) and [golangci-lint.yaml](https://github.com/kyma-project/compass-manager/blob/main/.github/workflows/golangci-lint.yaml).
- `pre-compass-manager-presubmit-scanner` - Triggered with a PR. It checks if the repository doesn't contain any vulnerabilities. For more information and the configuration, read [Kyma Security Scanner](https://github.tools.sap/kyma/security-scanners#readme).
- `post-main-compass-manager-build` - Triggered after the merge. Rebuilds the image and pushes it to the registry. For the configuration, check [build.yaml](https://github.com/kyma-project/test-infra/blob/main/prow/jobs/kyma-project/compass-manager/build.yaml).
- `Daily Markdown Link Check` - Runs Markdown link check every day at 05:00 AM. For the configuration, see [daily-markdown-link-check.yamk](https://github.com/kyma-project/compass-manager/blob/main/.github/workflows/daily-markdown-link-check.yaml).