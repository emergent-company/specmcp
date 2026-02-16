# App Entity Implementation Guide

## Overview

SpecMCP now fully supports **monorepo architectures** with the `App` and `DataModel` entities. This replaces the old `Service` entity with a cleaner model for deployable applications.

## Key Entities

### App
Represents a deployable application unit in your monorepo.

**Fields:**
- `name` - Unique identifier (kebab-case)
- `app_type` - Type: `frontend`, `backend`, `mobile`, `desktop`, `cli`, `library`
- `platform` - Target platforms (e.g., `["web"]`, `["ios", "android"]`)
- `root_path` - Root directory (e.g., `apps/web`, `services/auth`)
- `tech_stack` - Technologies (e.g., `["react", "typescript"]`)
- `instructions` - Setup, build, run, test instructions
- `deployment_target` - Where it deploys (e.g., `vercel`, `kubernetes`)
- `entry_point` - Main entry file
- `port` - Local development port
- `dependencies` - Key external dependencies

### DataModel
Represents shared data structures across apps.

**Fields:**
- `name` - Model name (e.g., `User`, `Organization`)
- `platform` - Where defined (e.g., `["go", "typescript"]`)
- `file_path` - Primary definition file
- `go_type`, `ts_type`, `swift_type` - Language-specific names
- `fields` - Key field names
- `persistence` - Storage: `database`, `memory`, `cache`, `none`

## Relationships

### App Relationships

| Relationship | Direction | Purpose |
|--------------|-----------|---------|
| `belongs_to_app` | Context/UIComponent/Action/APIContract → App | Entity owned by app |
| `scoped_to_app` | Change/Spec/Design/Task → App | Workflow scoped to app(s) |
| `depends_on_app` | App → App | Runtime dependency |
| `provides_model` | App → DataModel | App owns model |
| `consumes_model` | App → DataModel | App uses model |
| `exposes_api` | App → APIContract | App exposes API |
| `uses_pattern` | App → Pattern | App uses pattern |

## Creating Apps

Use `spec_artifact` tool with `artifact_type: "app"`:

```json
{
  "change_id": "setup-monorepo",
  "artifact_type": "app",
  "content": {
    "name": "web-frontend",
    "app_type": "frontend",
    "platform": ["web"],
    "root_path": "apps/web",
    "tech_stack": ["react", "typescript", "vite"],
    "instructions": "npm install && npm run dev",
    "deployment_target": "vercel",
    "entry_point": "src/main.tsx",
    "port": 3000
  }
}
```

## Creating DataModels

```json
{
  "change_id": "add-auth",
  "artifact_type": "data_model",
  "content": {
    "name": "User",
    "platform": ["go", "typescript"],
    "file_path": "models/user.go",
    "go_type": "User",
    "ts_type": "IUser",
    "fields": ["id", "email", "password_hash", "roles"],
    "persistence": "database"
  }
}
```

## Linking Apps and Models

### App Provides a Model

When creating an App, you can specify which models it provides:

The App that defines the model should have a `provides_model` relationship created automatically when you create the DataModel within that App's change context.

### App Consumes a Model

When creating entities that belong to an app and use models:

```json
{
  "artifact_type": "context",
  "content": {
    "name": "user-profile-screen",
    "app_id": "web-frontend",  // belongs to this app
    "consumed_model_ids": ["user-model-id"]  // uses this model
  }
}
```

## Cross-App Changes

When a change affects multiple apps, use `scoped_to_apps`:

```json
{
  "artifact_type": "spec",
  "content": {
    "name": "notification-spec",
    "scoped_to_apps": ["web-frontend", "mobile-app", "notification-service"]
  }
}
```

## Cross-App Scenarios

Scenarios can span contexts from different apps:

```yaml
Scenario: "user-completes-purchase"
  has_step:
    - "Select product"
      occurs_in: Context "product-catalog" (belongs_to_app: "web-frontend")
    
    - "Process payment"
      occurs_in: Context "payment-processor" (belongs_to_app: "payment-service")
    
    - "Send confirmation"
      occurs_in: Context "notification-queue" (belongs_to_app: "notification-service")
```

The system auto-computes which apps are involved by traversing:
`Scenario → ScenarioSteps → Contexts → Apps`

## Querying Apps

### Get App Details

```json
{
  "tool": "spec_get_app",
  "params": {
    "name": "web-frontend"  // or "id": "<app-id>"
  }
}
```

Returns:
- App properties
- API contracts it exposes
- DataModels it provides
- DataModels it consumes
- Contexts/Components/Actions that belong to it
- Changes/Specs scoped to it
- Apps it depends on
- Patterns it uses

### Search for Apps

```json
{
  "tool": "spec_search",
  "params": {
    "query": "frontend",
    "types": ["App"]
  }
}
```

## Example: Monorepo Setup

```javascript
// 1. Create backend API app
{
  "tool": "spec_artifact",
  "params": {
    "change_id": "setup-apps",
    "artifact_type": "app",
    "content": {
      "name": "auth-service",
      "app_type": "backend",
      "platform": ["go"],
      "root_path": "services/auth",
      "tech_stack": ["go", "grpc", "postgresql"],
      "port": 8080,
      "deployment_target": "kubernetes"
    }
  }
}

// 2. Create User model provided by auth-service
{
  "tool": "spec_artifact",
  "params": {
    "change_id": "setup-apps",
    "artifact_type": "data_model",
    "content": {
      "name": "User",
      "platform": ["go", "typescript"],
      "file_path": "services/auth/models/user.go",
      "fields": ["id", "email", "roles"],
      "persistence": "database"
    }
  }
}

// 3. Create frontend app that consumes User model
{
  "tool": "spec_artifact",
  "params": {
    "change_id": "setup-apps",
    "artifact_type": "app",
    "content": {
      "name": "web-frontend",
      "app_type": "frontend",
      "platform": ["web"],
      "root_path": "apps/web",
      "tech_stack": ["react", "typescript"],
      "consumed_model_ids": ["<user-model-id>"],  // uses User from auth-service
      "depends_on_apps": ["<auth-service-id>"]    // runtime dependency
    }
  }
}
```

## Migration from Service

The old `Service` entity has been removed. If you have existing Services:

1. They're now represented as Apps with `app_type: "backend"`
2. Relationships changed:
   - `belongs_to_service` → `belongs_to_app`
   - `uses_model` → `consumes_model` (on App only)
   - Service's `exposes_api` moved to App

## Tools Updated

### Query Tools
- ✅ `spec_get_app` - Get app with all relationships
- ✅ `spec_get_data_model` - Get model with provider/consumers
- ❌ `spec_get_service` - **Removed**

### Search
- ✅ `spec_search` now includes `App` and `DataModel` types

### Artifact Creation
- ✅ `spec_artifact` supports `artifact_type: "app"`
- ✅ `spec_artifact` supports `artifact_type: "data_model"`

## Best Practices

1. **Always ask which apps are affected** when starting a change
2. **Use `scoped_to_apps`** on Changes, Specs, Designs, and Tasks
3. **Link contexts/components/actions to apps** using `app_id`
4. **Define data models** and link them to provider/consumer apps
5. **Declare app dependencies** for runtime dependencies
6. **Use patterns at app level** to enforce consistency

## Template Pack

The App and DataModel entities are registered in `/root/specmcp/templates/specmcp-pack.json`.

Run `task seed` to register them with Emergent if you haven't already.

## Next Steps

1. Verify MCP client supports prompts/resources
2. Add interactive prompts for app creation
3. Enhance `spec_status` to show app-level information
4. Add app-level pattern enforcement
