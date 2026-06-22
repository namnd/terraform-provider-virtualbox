# Terraform Provider for VirtualBox

A [Terraform](https://www.terraform.io) provider for managing [VirtualBox](https://www.virtualbox.org/) resources. Built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.25
- [VirtualBox](https://www.virtualbox.org/) installed on the host where the provider runs

## Using the Provider

```terraform
terraform {
  required_providers {
    virtualbox = {
      source = "registry.terraform.io/namnd/virtualbox"
    }
  }
}

provider "virtualbox" {
  # example configuration here
}
```

See [`examples/provider/provider.tf`](examples/provider/provider.tf) for the provider configuration example used in documentation.

## Building the Provider

1. Clone the repository
2. Enter the repository directory
3. Build the provider using the Go `install` command:

```shell
go install
```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `make generate`.

In order to run the full suite of acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Examples

The [`examples/`](examples/) directory contains Terraform configuration used for documentation generation. Currently it includes:

- `provider/provider.tf` — provider configuration example for the provider index page

Additional resource and data source examples can be added under `examples/resources/` and `examples/data-sources/` as they are implemented.