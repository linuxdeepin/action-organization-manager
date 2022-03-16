# action-organization-manager

github actions for organization manage

## Features

1. Features switch (Wiki,Issues...)
2. Branchs protection

## Input

```yaml
  app_id:
    description: "github app id"
    required: true
  installation_id:
    description: "github app installation id"
    required: true
  private_key:
    description: "github app private key"
    required: true
  config_file:
    description: "manager config file"
    required: true
```

## Uses

1. Create and Install GitHub App in organization settings
1. Get AppID (App setting -> General) and InstallationID (App setting -> Advanced -> Recent Deliveries -> Payload)
1. Generate GitHub App Private Key and upload to organization secrets
1. Add `.github/workflows/org-mgr.yml` to organization repository

    ```yaml
    name: organization-manager
    on:
    push:
        paths: ["organization.yaml", ".github/workflows/org-mgr.yml"]

    jobs:
        job:
            name: organization-manager
            runs-on: ubuntu-latest
            steps:
            - uses: myml/action-organization-manager@v0.0.3
                with:
                app_id: $app_id
                installation_id: $installation_id
                private_key: ${{ secrets.APP_PRIVATE_KEY }}
                config_file: organization.yaml
    ```

1. Add `organization.yaml` config file to organization repository

    ```yaml
    organization: $organization_name
    settings:
    - repositories: [$repositories_name or regular expression]
        features:
        issues:
            enable: true
        wiki:
            enable: true
        branches:
          $branche_name:
            dismiss_stale_reviews: true
            enforce_admins: true
            required_approving_review_count: 1
            required_status_checks:
            require_review: true
            strict: true
            allow_force_pushes: false
            allow_deletions: false
    ```
