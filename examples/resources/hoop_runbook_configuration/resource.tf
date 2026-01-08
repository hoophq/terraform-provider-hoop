
# Runbooks configuration. Public Repositories
resource "hoop_runbook_configuration" "hoop-public-runbooks" {
  git_url         = "https://github.com/hoophq/runbooks.git"
  git_hook_ttl    = 0
  git_user        = ""
  git_password    = ""
  ssh_user        = ""
  ssh_key         = ""
  ssh_keypass     = ""
  ssh_known_hosts = ""
}


# Runbooks configuration. Basic Credentials
resource "hoop_runbook_configuration" "hoop-public-runbooks" {
  git_url         = "https://github.com/your-org/your-repo"
  git_hook_ttl    = 0
  git_user        = "oauth2"
  git_password    = "your-personal-access-token"
  ssh_user        = ""
  ssh_key         = ""
  ssh_keypass     = ""
  ssh_known_hosts = ""
}

# Runbooks configuration. SSH Private Keys
resource "hoop_runbook_configuration" "hoop-public-runbooks" {
  git_url         = "git@github.com:your-org/your-repo.git"
  git_hook_ttl    = 0
  git_user        = ""
  git_password    = ""
  ssh_user        = ""
  ssh_key         = file("${path.module}/ssh_key.pem")
  ssh_keypass     = "your-ssh-key-passphrase"
  ssh_known_hosts = file("${path.module}/known_hosts")
}
