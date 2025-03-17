terraform {
  required_providers {
    hoop = {
      source  = "terraform.local/local/hoop"
      version = "0.0.1"
    }
  }
}

provider "hoop" {
  api_key = "test-api-key"
  api_url = "http://localhost:8009/api"
}

resource "hoop_connection" "test" {
  name     = "test-connection"
  type     = "database"
  subtype  = "postgres"
  agent_id = "test-agent"
  secrets = {
    user     = "testuser"
    pass     = "password"
    db       = "testdb"
    host     = "localhost"
    port     = "5432"
  }
}