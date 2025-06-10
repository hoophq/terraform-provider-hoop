# Terraform Provider Hoop

This provider leverages the hoop API to manage resources in the Hoop platform.
It's a Terraform provider that allows you to create, read, update, and delete resources in Hoop.

## Supported Resources

- [x] Connections
- [x] Plugin Connection

## Documentation

Refer to [./docs](./docs) or the [Hoop Terraform Provider Documentation](https://registry.terraform.io/providers/hoophq/hoop/latest/docs) for detailed documentation on how to use the provider, including examples and configuration options.

## Development

This provider is built using the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).
It was scaffolded using the [Terraform Provider Scaffolding](https://github.com/hashicorp/terraform-provider-scaffolding-framework) template repository.

### Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.23

### Testing the Provider

To test the provider, you can run the following command:

```shell
make test
```

1. To work with a live Hoop instance, spin up a local instance of the Hoop API server:

- Use the development server in the [hoophq/hoop repository](https://github.com/hoophq/hoop/blob/main/DEV.md).
- Use the [docker-compose setup](https://hoop.dev/docs/setup/deployment/docker-compose)

2. Obtain the organization id to [setup your API KEY](https://hoop.dev/docs/setup/apis/api-key).
3. Restart your development server with the API KEY set
4. Open a new terminal and use the script `./scripts/terraform` to run the provider with the local Hoop API server.

```sh
mv ./dev/main/main.tf-sample ./dev/main/main.tf
./scripts/terraform plan -chdir=dev/main plan
```

> *The script will use the `$HOME/go/bin` folder by default to install the provider binary.*

It will build and install the provider, and then run the `plan` command against the local Hoop API server.
If you need to clean the state, remove the local files in this folder:

```sh
rm -rf ./dev/main/terraform.tfstate*
```

To clean up resources from the local Hoop API server, the command line could be used to manage the resources directly.

```sh
hoop admin delete conn bash-console
```

### Updating Documentation

This project uses the `tfplugindocs` tool to generate documentation for the provider.
The `tfplugindocs` tool will automatically include schema-based descriptions, if present in a data source, provider, or resource's schema. The `schema.Schema` type's `Description` field describes the data source, provider, or resource itself. Each attribute's or block's Description field describes that particular attribute or block. These descriptions should be tailored to practitioner usage and include any caveats or value expectations, such as special syntax.

To generate the documentation, you can run the following command:

```sh
make generate
```

#### Example File Documentation

The `tfplugindocs` tool will automatically include Terraform configuration examples from files with the following naming conventions:

- **Provider:** `examples/provider/provider.tf`
- **Resources:** `examples/resources/TYPE/resource.tf`
- **Data Sources:** `examples/data-sources/TYPE/data-source.tf`
- **Functions:** `examples/functions/TYPE/function.tf`

Replace `TYPE` with the name of the resource, data source, or function. For example: `examples/resources/hoop_connection/resource.tf`.

#### Example Import Documentation

The `tfplugindocs` tool automatically will include Terraform import examples for resources with the file naming convention `examples/resources/TYPE/import.sh`.

