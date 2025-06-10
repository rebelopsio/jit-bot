# Slack Setup Guide

This guide covers setting up Slack integration for the JIT Access Tool.

## Overview

The JIT Access Tool integrates with Slack to provide a user-friendly interface for requesting and approving temporary access to EKS clusters. Users can interact with the system using slash commands in Slack channels.

## Prerequisites

- Slack workspace with administrative privileges
- Ability to create and configure Slack apps
- Public endpoint for the JIT server (for webhook delivery)

## 1. Create a Slack App

### 1.1 Basic App Setup

1. **Go to Slack API Portal**:
   - Visit [https://api.slack.com/apps](https://api.slack.com/apps)
   - Click "Create New App"
   - Choose "From scratch"

2. **Configure Basic Information**:
   - **App Name**: `JIT Access Tool`
   - **Development Slack Workspace**: Select your workspace
   - Click "Create App"

### 1.2 App Manifest (Alternative)

You can also create the app using this manifest:

```json
{
  "display_information": {
    "name": "JIT Access Tool",
    "description": "Just-In-Time access management for AWS EKS clusters",
    "background_color": "#2c3e50"
  },
  "features": {
    "bot_user": {
      "display_name": "JIT Access Bot",
      "always_online": true
    },
    "slash_commands": [
      {
        "command": "/jit",
        "url": "https://your-domain.com/slack/commands",
        "description": "Manage JIT access to EKS clusters",
        "usage_hint": "request <cluster> <duration> <reason>"
      }
    ]
  },
  "oauth_config": {
    "scopes": {
      "bot": [
        "commands",
        "chat:write",
        "chat:write.public",
        "users:read",
        "users:read.email"
      ]
    }
  }
}
```

## 2. Configure Slash Commands

### 2.1 Create the `/jit` Command

1. **Navigate to Slash Commands**:
   - In your app settings, go to "Features" → "Slash Commands"
   - Click "Create New Command"

2. **Configure the Command**:
   - **Command**: `/jit`
   - **Request URL**: `https://your-domain.com/slack/commands`
   - **Short Description**: "Manage JIT access to EKS clusters"
   - **Usage Hint**: `request <cluster> <duration> <reason>`
   - Click "Save"

### 2.2 Command Examples

The `/jit` command supports various subcommands:

```bash
# Request access
/jit request prod-east-1 2h "Deploy hotfix for critical bug"

# Request with specific permissions
/jit request staging-west-2 4h "Testing new feature" --permissions=edit --namespaces=default,testing

# Approve a request
/jit approve jit-user123-1234567890 "Approved for emergency deployment"

# List requests
/jit list mine                    # Your requests
/jit list                        # All requests (admin only)

# Get help
/jit help
```

## 3. Configure Bot Permissions

### 3.1 OAuth Scopes

1. **Navigate to OAuth & Permissions**:
   - Go to "Features" → "OAuth & Permissions"
   - Scroll down to "Scopes"

2. **Add Bot Token Scopes**:
   - `commands` - Execute slash commands
   - `chat:write` - Send messages as the bot
   - `chat:write.public` - Send messages to public channels
   - `users:read` - Read user profile information
   - `users:read.email` - Read user email addresses
   - `im:write` - Send direct messages to users

### 3.2 Install to Workspace

1. **Install the App**:
   - Scroll up to "OAuth Tokens for Your Workspace"
   - Click "Install to Workspace"
   - Authorize the requested permissions

2. **Copy Bot Token**:
   - After installation, copy the "Bot User OAuth Token"
   - This starts with `xoxb-`
   - Store this securely - you'll need it for Kubernetes secrets

## 4. Configure Event Subscriptions (Optional)

### 4.1 Enable Events

1. **Navigate to Event Subscriptions**:
   - Go to "Features" → "Event Subscriptions"
   - Toggle "Enable Events" to On

2. **Set Request URL**:
   - **Request URL**: `https://your-domain.com/slack/events`
   - Slack will verify this URL

### 4.2 Subscribe to Bot Events

Add these bot events:
- `message.im` - Direct messages to the bot
- `app_mention` - When the bot is mentioned

## 5. Configure Interactive Components

### 5.1 Enable Interactivity

1. **Navigate to Interactivity & Shortcuts**:
   - Go to "Features" → "Interactivity & Shortcuts"
   - Toggle "Interactivity" to On

2. **Set Request URL**:
   - **Request URL**: `https://your-domain.com/slack/interactive`

### 5.2 Add Shortcuts (Optional)

Create shortcuts for common actions:

```json
{
  "name": "Request EKS Access",
  "type": "global",
  "callback_id": "request_access_shortcut",
  "description": "Quick access request form"
}
```

## 6. Security Configuration

### 6.1 Signing Secret

1. **Get Signing Secret**:
   - Go to "Settings" → "Basic Information"
   - Scroll down to "App Credentials"
   - Copy the "Signing Secret"
   - Store this securely for request verification

### 6.2 Request Verification

The JIT server verifies all incoming requests from Slack using the signing secret. This ensures requests are authentic.

Example verification process:
```go
func verifySlackRequest(r *http.Request, signingSecret string) bool {
    timestamp := r.Header.Get("X-Slack-Request-Timestamp")
    signature := r.Header.Get("X-Slack-Signature")
    
    // Verify timestamp is within 5 minutes
    ts, _ := strconv.ParseInt(timestamp, 10, 64)
    if math.Abs(float64(time.Now().Unix()-ts)) > 300 {
        return false
    }
    
    // Verify signature
    body, _ := ioutil.ReadAll(r.Body)
    baseString := "v0:" + timestamp + ":" + string(body)
    
    h := hmac.New(sha256.New, []byte(signingSecret))
    h.Write([]byte(baseString))
    expectedSignature := "v0=" + hex.EncodeToString(h.Sum(nil))
    
    return signature == expectedSignature
}
```

## 7. User Management

### 7.1 User Role Mapping

Configure user roles in your Kubernetes ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: slack-user-roles
  namespace: jit-system
data:
  roles.yaml: |
    roles:
      admins:
        - U1234ADMIN1  # Slack User ID
        - U5678ADMIN2
      approvers:
        - U9012APPROVER1
        - U3456APPROVER2
        - U7890APPROVER3
      requesters:
        - U1111USER1
        - U2222USER2
        # Add more users as needed
```

### 7.2 Get Slack User IDs

To find Slack user IDs:

```bash
# Method 1: Use Slack API
curl -H "Authorization: Bearer xoxb-your-bot-token" \
     "https://slack.com/api/users.list"

# Method 2: In Slack, right-click user → Copy member ID

# Method 3: Use the bot to get your own ID
/jit whoami
```

## 8. Channel Configuration

### 8.1 Dedicated JIT Channel

Create a dedicated channel for JIT access:

1. **Create Channel**:
   - Create a public channel: `#jit-access`
   - Invite the JIT bot to the channel
   - Pin important information (help, policies)

2. **Channel Guidelines**:
   ```
   📋 JIT Access Guidelines
   
   🔹 Use /jit request for access requests
   🔹 Include clear business justification
   🔹 Follow the principle of least privilege
   🔹 Maximum access duration: 4h for prod, 8h for staging
   
   Commands:
   • /jit help - Show all commands
   • /jit request <cluster> <duration> <reason>
   • /jit approve <request-id> [comment]
   • /jit list [mine]
   ```

### 8.2 Private Admin Channel

Create a private channel for administrators:

1. **Create Private Channel**: `#jit-admin`
2. **Invite**: Only JIT administrators and the bot
3. **Purpose**: Administrative commands, monitoring, alerts

## 9. Testing the Integration

### 9.1 Test Basic Commands

```bash
# Test help command
/jit help

# Test request (should work for any user with requester role)
/jit request dev-west-2 1h "Testing integration"

# Test list (should show the request)
/jit list mine

# Test approval (requires approver role)
/jit approve <request-id> "Test approval"
```

### 9.2 Test Error Scenarios

```bash
# Test invalid cluster
/jit request nonexistent-cluster 1h "Test"

# Test invalid duration
/jit request dev-west-2 25h "Test"

# Test unauthorized approval
/jit approve some-request-id "Test" # (as non-approver)
```

## 10. Monitoring and Logging

### 10.1 Slack App Logs

Monitor your app's usage:

1. **Go to App Settings**:
   - Navigate to "Features" → "OAuth & Permissions"
   - Check "Recent Activity" for API calls

2. **Event Logs**:
   - Monitor slash command usage
   - Track failed requests

### 10.2 Application Logs

The JIT server logs all Slack interactions:

```bash
# View Slack-related logs
kubectl logs deployment/jit-server -n jit-system | grep slack

# Monitor command usage
kubectl logs deployment/jit-server -n jit-system | grep "jit request"
```

## 11. Best Practices

### 11.1 Security

- **Verify all requests** using the signing secret
- **Use HTTPS** for all webhook URLs
- **Rotate tokens** regularly
- **Monitor for suspicious activity**

### 11.2 User Experience

- **Provide clear error messages**
- **Use interactive components** for complex workflows
- **Send confirmations** for important actions
- **Include helpful context** in responses

### 11.3 Compliance

- **Log all access requests** and approvals
- **Include business justification** in requests
- **Set appropriate access durations**
- **Regular access reviews**

## 12. Troubleshooting

### 12.1 Common Issues

1. **Command not responding**:
   ```bash
   # Check if webhook URL is accessible
   curl -X POST https://your-domain.com/slack/commands
   
   # Check Slack app logs
   kubectl logs deployment/jit-server -n jit-system
   ```

2. **Permission denied errors**:
   ```bash
   # Verify user roles
   kubectl get configmap slack-user-roles -n jit-system -o yaml
   
   # Check RBAC configuration
   kubectl logs deployment/jit-server -n jit-system | grep "permission denied"
   ```

3. **Invalid signature errors**:
   ```bash
   # Verify signing secret is correct
   kubectl get secret slack-config -n jit-system -o yaml
   
   # Check timestamp skew
   kubectl logs deployment/jit-server -n jit-system | grep "timestamp"
   ```

### 12.2 Debug Mode

Enable debug logging for Slack integration:

```bash
kubectl set env deployment/jit-server SLACK_DEBUG=true -n jit-system
```

### 12.3 Testing Webhooks Locally

For local development, use ngrok to expose your local server:

```bash
# Install ngrok
npm install -g ngrok

# Expose local server
ngrok http 8080

# Update Slack app with ngrok URL
# Example: https://abc123.ngrok.io/slack/commands
```

## 13. Advanced Configuration

### 13.1 Custom Slack Blocks

Enhance user experience with rich message formatting:

```go
func createAccessRequestMessage(req *JITAccessRequest) slack.Message {
    return slack.Message{
        Blocks: []slack.Block{
            slack.NewSectionBlock(
                slack.NewTextBlockObject("mrkdwn", 
                    fmt.Sprintf("*New JIT Access Request*\n*User:* <@%s>\n*Cluster:* %s\n*Duration:* %s", 
                        req.UserID, req.Cluster, req.Duration)),
                nil, nil,
            ),
            slack.NewActionBlock("approve_deny",
                slack.NewButtonBlockElement("approve", req.ID, 
                    slack.NewTextBlockObject("plain_text", "Approve", false)),
                slack.NewButtonBlockElement("deny", req.ID,
                    slack.NewTextBlockObject("plain_text", "Deny", false)),
            ),
        },
    }
}
```

### 13.2 Workflow Integration

Integrate with Slack Workflow Builder for complex approval processes:

1. **Create Workflow**:
   - Access request triggers workflow
   - Automatic routing to appropriate approvers
   - Time-based escalation

2. **Webhook Integration**:
   - Workflow sends webhooks to JIT server
   - Server processes approval/denial
   - Updates request status in Kubernetes