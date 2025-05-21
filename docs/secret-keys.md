# 🔑 Secret Keys Management

## Overview
Secret Keys in GADS provide a flexible authentication system supporting multiple JSON Web Token (JWT) issuers through origin-based key management. This feature enables secure integration with various external authentication systems and service providers.

## 🔐 Core Functionality

### Origin-Based Authentication
- Configure unique secret keys for different application origins (e.g., mobile apps, web apps, external services)
- Support for multiple authentication sources within a single GADS instance
- Default key configuration for handling unknown origins

### JWT Claim Mapping
- Custom mapping of JWT claims to identify users
- Optional tenant identification for multi-tenant environments
- Flexible claim structure to adapt to different authentication providers

### Secure Key Management
- Generate secure cryptographic keys
- Update keys with full audit history
- Disable keys without deleting authentication records

## ⚙️ Administration Features

### Key Management Interface
- Add, edit, and disable secret keys
- Set default key for fallback authentication
- Configure user and tenant identifier claims
- Generate secure random keys

### Audit History
- Track all key-related operations with timestamps
- Record justifications for key changes and disabling
- Filter history by origin, action type, user, and date range
- Full accountability for security operations

## 📋 Common Use Cases

### Multi-Application Support
Configure different keys for various applications (iOS app, Android app, web portal) that connect to the same GADS instance.

### Authentication Provider Migration
Manage multiple authentication keys during transition periods between identity providers.

## 💻 Implementation Example

```javascript
// Example JWT payload structure
{
  "sub": "user123",           // User identifier (mapped with user_identifier_claim)
  "org_id": "tenant456",      // Tenant identifier (mapped with tenant_identifier_claim)
  "name": "John Doe",
  "email": "john@example.com",
  "exp": 1713897600,
  "iat": 1713811200,
  "iss": "https://auth.example.com"  // JWT issuer
}
```

## ⚠️ Security Considerations

1. **Key Rotation**
   - Regularly rotate keys as part of security best practices
   - Provide justification for all key changes for audit purposes

2. **Default Key Usage**
   - Be cautious when changing the default key
   - Existing tokens from unknown origins may stop working when default key changes

3. **Claim Selection**
   - Choose unique and stable claims for user identification
   - Ensure selected claims will be consistently present in all issued JWTs

## 🔄 Integration with Workspaces

Secret Keys can be used in conjunction with the Workspace feature to provide comprehensive access control:
- JWT claims can determine which workspace a user has access to
- Tenant identification enables multi-tenant isolation
- Different origins can have different workspace access patterns