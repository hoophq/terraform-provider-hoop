# Access Groups for different teams
# This file demonstrates how to create access groups to control
# which teams have access to specific database connections

# Development team access group - has access to development databases
resource "hoop_access_group" "dev_team" {
  group       = "dev_team"
  description = "Access group for development team members"

  # This group has access to all development connections
  # Note: These connections must be created beforehand or in the same configuration
  connections = [
    hoop_connection.postgres_dev.name,
    hoop_connection.mysql_dev.name
  ]
}

# Database administrators group - has access to all databases
resource "hoop_access_group" "db_admins" {
  group       = "db_admins"
  description = "Database administrators with access to all databases"

  # This group has access to all connections, both dev and production
  connections = [
    hoop_connection.postgres_dev.name,
    hoop_connection.mysql_dev.name,
    hoop_connection.postgres_prod.name,
    hoop_connection.mysql_prod.name
  ]
}

# Analytics team - has read-only access to specific databases
resource "hoop_access_group" "analytics" {
  group       = "analytics_team"
  description = "Data analysts with read access to specific databases"

  # This group only has access to production databases
  connections = [
    hoop_connection.postgres_prod.name,
    hoop_connection.mysql_prod.name
  ]
}

# Security team - focused on sensitive data monitoring
resource "hoop_access_group" "security" {
  group       = "security_team"
  description = "Security team with access to monitor sensitive databases"

  # This group has access to production databases for security monitoring
  connections = [
    hoop_connection.postgres_prod.name
  ]
}

# Output the created access groups for reference
output "access_groups" {
  value = {
    dev_team  = hoop_access_group.dev_team.group
    db_admins = hoop_access_group.db_admins.group
    analytics = hoop_access_group.analytics.group
    security  = hoop_access_group.security.group
  }
  description = "The access groups that have been created"
}
