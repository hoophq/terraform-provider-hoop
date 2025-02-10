variable "hoop_api_key" {
  type        = string
  description = "API Key for Hoop.dev"
  sensitive   = true
}

variable "api_url" {
  type        = string
  description = "API URL for Hoop.dev"
  default     = "http://localhost:8009/api"
}

variable "agent_id" {
  type        = string
  description = "Agent ID to use for connections"
}

# Mock database hosts
variable "database_hosts" {
  type = object({
    postgres = object({
      host = string
      port = string
    })
    mysql = object({
      host = string
      port = string
    })
    mysql_replica = object({
      host = string
      port = string
    })
    mongodb = object({
      connection_string = string
    })
    mongodb_replica = object({
      connection_string = string
    })
    mssql = object({
      host = string
      port = string
    })
    oracle = object({
      host = string
      port = string
      sid = string
    })
  })
}

# Credentials for updates
variable "database_credentials" {
  type = object({
    basic = object({
      user = string
      password = string
    })
    replica = object({
      user = string
      password = string
    })
    oracle = object({
      user = string
      password = string
    })
    oracle_enterprise = object({
      user = string
      password = string
    })
  })
}
