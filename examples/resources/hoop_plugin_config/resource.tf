# Copyright (c) HashiCorp, Inc.

# Slack plugin configuration.
resource "hoop_plugin_config" "slack" {
  plugin_name = "slack"
  config = {
    SLACK_BOT_TOKEN = "xoxb-213..."
    SLACK_APP_TOKEN = "xapp-1-A0..."
  }
}

# Runbooks plugin configuration. Public Repositories
resource "hoop_plugin_config" "runbooks" {
  plugin_name = "runbooks"
  config = {
    GIT_URL = "https://github.com/your-org/your-public-repo"
  }
}

# Runbooks plugin configuration. Basic Credentials
resource "hoop_plugin_config" "runbooks" {
  plugin_name = "runbooks"
  config = {
    GIT_URL      = "https://github.com/your-org/your-repo"
    GIT_USER     = "oauth2" # optional
    GIT_PASSWORD = "your-personal-access-token"
  }
}

# Runbooks plugin configuration. SSH Private Keys
resource "hoop_plugin_config" "runbooks" {
  plugin_name = "runbooks"
  config = {
    GIT_URL             = "git@github.com:your-org/your-repo.git"
    GIT_SSH_KEY         = file("${path.module}/ssh_key.pem")
    GIT_SSH_USER        = "ssh-user"                         # optional
    GIT_SSH_KEYPASS     = "your-ssh-key-passphrase"          # optional
    GIT_SSH_KNOWN_HOSTS = file("${path.module}/known_hosts") # optional
  }
}
