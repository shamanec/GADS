# Workspace Management

## Overview
GADS implements a workspace-based organization system that enables device management and access control. Workspaces allow administrators to segment devices and control user access to specific device groups.

## Key Features

### Workspace Types
- **Default Workspace**: Automatically created and cannot be deleted. Houses all devices and users that aren't explicitly assigned to other workspaces
- **Custom Workspaces**: User-created workspaces for organizing devices and controlling access

### Access Control
- **Admin Users**: Have access to all workspaces and can manage workspace assignments
- **Regular Users**: Can only access devices in their assigned workspaces
- **Device Assignment**: Each device belongs to exactly one workspace
- **User Assignment**: Users can be assigned to multiple workspaces

### Management Features
- Create, update, and delete workspaces
- Assign devices to workspaces
- Manage user access to workspaces
- Search and filter workspaces
- Pagination support for large installations

## Usage

### Creating a Workspace
1. Navigate to the Admin Dashboard
2. Select the "Workspaces" tab
3. Click "Add Workspace"
4. Fill in the required fields:
   - Name (must be unique)
   - Description
5. Click "Create"

### Managing Devices in Workspaces
1. Go to the Admin Dashboard
2. Select the "Devices" tab
3. For new devices:
   - Fill in the device details (choosing the workspace)
   - Click "Add Device"
4. For existing devices:
   - Locate the device card
   - Select a different workspace from the dropdown
   - Click "Update Device"

### Managing User Access
1. Access the Admin Dashboard
2. Select the "Users" tab
3. To modify workspace access:
   - Locate the user card
   - Select or deselect workspaces from the multi-select dropdown
   - Click "Update User"

### Viewing Workspace Devices
1. Go to the Device Selection screen
2. Use the workspace selector dropdown at the left
3. The device list will automatically filter to show only devices from the selected workspace
4. Additional filters available:
   - All devices
   - Android only
   - iOS only
   - Search by device name/details

## Technical Details

### API Endpoints

#### Workspace Management
- `GET /admin/workspaces`: List all workspaces (paginated)
- `POST /admin/workspaces`: Create new workspace
- `PUT /admin/workspaces`: Update existing workspace
- `DELETE /admin/workspaces/:id`: Delete workspace

#### User Workspaces
- `GET /workspaces`: Get workspaces accessible to current user

### Database Schema
```go
type Workspace struct {
    ID          string
    Name        string
    Description string
    IsDefault   bool
}

type User struct {
    // Other fields...
    WorkspaceIDs []string
}

type Device struct {
    // Other fields...
    WorkspaceID  string
}
```

## Limitations

- The default workspace cannot be deleted
- Workspaces with assigned devices or users cannot be deleted
- Workspace names must be unique across the system

## Best Practices

1. Use meaningful workspace names that reflect your organization's structure
2. Regularly review workspace assignments
3. Maintain clear documentation of workspace purpose and contents
4. Use the default workspace sparingly, primarily for legacy support
5. Implement a consistent naming convention for workspaces
