# Copyright (c) HashiCorp, Inc.

# The import reads every block the gateway holds into state: the gateway
# cannot say which ones Terraform owns. Declare the blocks to keep; the next
# apply stops the proxy of any imported block the configuration leaves out.
terraform import hoop_server_misc_config.this server
