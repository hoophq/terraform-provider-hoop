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

# Database host configurations
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
    mongodb = object({
      host = string
    })
  })

  description = "Database host configurations for test connections"
}
