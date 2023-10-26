[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/compass-manager)](https://api.reuse.software/info/github.com/kyma-project/compass-manager)

# Compass Manager

## Overview
Compass Manager **will be** a new Control Plane component. Build using [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework. 

It's main responsibilities **will be**:
- registering Kyma runtimes in the Compass Director
- creating secret on client cluster with the Compass Runtime configuration

## Prerequisites

- Access to a k8s cluster.
- [k3d](https://k3d.io) to get a local cluster for testing, or run against a remote cluster.
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [kubebuilder](https://book.kubebuilder.io/)

## Installation

> Explain the steps to install your project. Create an ordered list for each installation task.
>
> If it is an example README.md, describe how to build, run locally, and deploy the example. Format the example as code blocks and specify the language, highlighting where possible. Explain how you can validate that the example ran successfully. For example, define the expected output or commands to run which check a successful deployment.
>
> Add subsections (H3) for better readability.

## Usage

> Explain how to use the project. You can create multiple subsections (H3). Include the instructions or provide links to the related documentation.

## Development

> Add instructions on how to develop the project or example. It must be clear what to do and, for example, how to trigger the tests so that other contributors know how to make their pull requests acceptable. Include the instructions or provide links to related documentation.

## Troubleshooting

> List potential issues and provide tips on how to avoid or solve them. To structure the content, use the following sections:
>
> - **Symptom**
> - **Cause**
> - **Remedy**
