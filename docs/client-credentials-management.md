# 🔑 Client Credentials Management

## Overview

GADS provides a user-friendly web interface for managing OAuth2 client credentials. This interface allows administrators to create, view, edit, and revoke credentials that are used for authenticating Appium test automation clients.

## 📍 Accessing the Credentials Interface

1. Log in to the GADS web interface as an administrator
2. Navigate to the **Admin** panel
3. Select **Client Credentials** from the menu

## ✨ Creating New Credentials

### Step 1: Start Creation Process

Click the **Create New Credential** button in the top-right corner of the Client Credentials page.

### Step 2: Fill in the Form

You'll be presented with a form containing two fields:

- **Name** (Required): A descriptive name for the credential
  - Example: "CI/CD Pipeline - Android Tests"
  - This helps identify the purpose of each credential
  
- **Description** (Optional): Additional details about the credential usage
  - Example: "Used by Jenkins for nightly Android regression tests"
  - Helpful for documentation and audit purposes

### Step 3: Submit the Form

Click the **Create Credential** button to generate the new credentials.

## 🎉 Credential Creation Success

After successful creation, a dialog window will display the following information:

### ⚠️ Important Warning
A prominent warning alerts you that **the client secret will not be shown again**. It's crucial to save this information immediately.

### Generated Information

1. **Client ID**
   - Format: `<prefix>_<timestamp>_<random_suffix>`
   - Example: `gads_1704123456_abc12345`
   - This ID is used to identify the client application

2. **Client Secret**
   - A secure, randomly generated string
   - Can be toggled between visible/hidden using the eye icon
   - **This is shown only once** - save it securely!

3. **Tenant**
   - The tenant/workspace associated with the credential
   - Automatically assigned based on your current context

4. **Appium Capabilities**
   - Pre-formatted JSON showing how to use the credential
   - Example:
   ```json
   {
     "gads:clientSecret": "your-generated-secret-here"
   }
   ```

### Copying Information

Each field has a copy button (📋) allowing you to easily copy:
- Individual fields (Client ID, Secret, Tenant)
- The complete Appium capabilities JSON

## 📊 Managing Existing Credentials

### Viewing Credentials List

The main credentials page displays a table with:
- **Name**: The credential's descriptive name
- **Client ID**: The unique identifier
- **Description**: Additional details (if provided)
- **Created Date**: When the credential was created
- **Last Used**: When the credential was last used for authentication
- **Status**: Active or Revoked

### Searching Credentials

Use the search box to filter credentials by:
- Name
- Client ID
- Description

### Editing Credentials

1. Click the **Edit** button next to a credential
2. Update the Name or Description
3. Click **Update Credential** to save changes

**Note**: You cannot change the Client ID or regenerate the secret

### Revoking Credentials

1. Click the **Revoke** button next to a credential
2. Confirm the revocation in the dialog
3. The credential will be immediately disabled

**Warning**: Revocation is permanent. Any applications using this credential will lose access.

## 🛡️ Security Best Practices

1. **Immediate Storage**
   - Copy and securely store the client secret immediately after creation
   - Use a password manager or secure vault service

2. **Descriptive Naming**
   - Use clear, descriptive names that indicate the credential's purpose
   - Include environment information (e.g., "Production Android Tests")

3. **Regular Audits**
   - Review the "Last Used" column to identify unused credentials
   - Revoke credentials that are no longer needed

## 🔄 Integration Workflow

1. **Create** credentials through the web interface
2. **Save** the client secret securely
3. **Configure** your Appium tests with the credential
4. **Monitor** usage through the interface

For information on how to use these credentials in your Appium tests, see the [Appium Credentials Documentation](./appium-credentials.md).