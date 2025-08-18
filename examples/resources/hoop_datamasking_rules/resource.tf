# Copyright (c) HashiCorp, Inc.

# Data Masking Rules configuration with supported entity types.
resource "hoop_datamasking_rules" "rule1" {
  name                = "Example Rule 1"
  description         = "This is an example datamasking rule 1."
  score_threshold     = 0.51
  connection_ids      = [hoop_connection.bash-console.id]
  custom_entity_types = []
  supported_entity_types = [
    {
      name = "PII"
      entity_types = [
        "EMAIL_ADDRESS",
        "PHONE_NUMBER"
      ]
    },
    {
      name = "PII-2"
      entity_types = [
        "PERSON",
        "URL"
      ]
    }
  ]
}

# Data Masking Rules configuration with custom entity types.
resource "hoop_datamasking_rules" "rule2" {
  name            = "Example Rule 2"
  description     = "This is an example datamasking rule 2."
  score_threshold = 0
  connection_ids  = [hoop_connection.bash-console.id]
  supported_entity_types = [
    {
      name = "BASELINE PII"
      entity_types = [
        "EMAIL_ADDRESS",
        "PHONE_NUMBER"
      ]
    },
    {
      name = "BASELINE PII-2"
      entity_types = [
        "PERSON",
        "URL"
      ]
    }
  ]

  custom_entity_types = [
    {
      name : "CUSTOM PII",
      score : 0.81,
      regex : "example.com.+",
    },
    {
      name : "CUSTOM PII-2",
      score : 0.65,
      deny_list : ["example.org", "sample.org"],
    }
  ]
}
