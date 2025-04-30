# Runbooks Path Configuration
# This file demonstrates how to configure runbooks paths for connections

# PostgreSQL development database runbooks path
resource "hoop_runbooks_path" "postgres_dev_runbooks" {
  connection_id   = hoop_connection.postgres_dev.id
  connection_name = hoop_connection.postgres_dev.name
  path            = "/runbooks/dev/postgres"

  depends_on = [hoop_connection.postgres_dev]
}

# PostgreSQL production database runbooks path
resource "hoop_runbooks_path" "postgres_prod_runbooks" {
  connection_id   = hoop_connection.postgres_prod.id
  connection_name = hoop_connection.postgres_prod.name
  path            = "/runbooks/prod/postgres"

  depends_on = [hoop_connection.postgres_prod]
}

# MySQL development database runbooks path
resource "hoop_runbooks_path" "mysql_dev_runbooks" {
  connection_id   = hoop_connection.mysql_dev.id
  connection_name = hoop_connection.mysql_dev.name
  path            = "/runbooks/dev/mysql"

  depends_on = [hoop_connection.mysql_dev]
}

# MySQL production database runbooks path
resource "hoop_runbooks_path" "mysql_prod_runbooks" {
  connection_id   = hoop_connection.mysql_prod.id
  connection_name = hoop_connection.mysql_prod.name
  path            = "/runbooks/prod/mysql"

  depends_on = [hoop_connection.mysql_prod]
}

# Output the configured runbooks paths for reference
output "runbooks_paths" {
  value = {
    postgres_dev  = hoop_runbooks_path.postgres_dev_runbooks.path
    postgres_prod = hoop_runbooks_path.postgres_prod_runbooks.path
    mysql_dev     = hoop_runbooks_path.mysql_dev_runbooks.path
    mysql_prod    = hoop_runbooks_path.mysql_prod_runbooks.path
  }
  description = "The configured runbooks paths for each connection"
} 
