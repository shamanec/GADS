# 🎯 GADS WebRTC TURN Server - Deployment and Configuration Guide

**A comprehensive guide to deploying and configuring a TURN server for GADS WebRTC connectivity.**

---

## 📋 Table of Contents

- [Context and Problem](#context-and-problem)
- [Technical Fundamentals](#technical-fundamentals)
- [Prerequisites](#prerequisites)
- [Deployment Step-by-Step](#deployment-step-by-step)
- [GADS Hub Configuration](#gads-hub-configuration)
- [Validation and Testing](#validation-and-testing)
- [Troubleshooting](#troubleshooting)
- [Security Best Practices](#security-best-practices)
- [Alternative setup on Hetzner using coturn](#simple-hetzner-turn-server-setup-using-coturn)
- [FAQ](#faq)
- [References](#references)

---

## 🔍 Context and Problem

### Why TURN is Necessary

WebRTC is designed to establish peer-to-peer connections between devices. However, real-world network topologies present significant challenges:

- **Symmetric NAT**: Some NAT devices change the external port mapping for each unique destination, making direct connections impossible.
- **Carrier-Grade NAT (CGN)**: ISPs using CGN place multiple users behind a single public IP, severely limiting peer-to-peer connectivity.
- **Corporate Firewalls**: Restrictive firewall policies block UDP traffic or only allow specific ports.

**Impact**: Without TURN relay servers, approximately **8-15% of WebRTC connections fail** due to these network restrictions.

### When TURN is Used

WebRTC connection establishment follows the **ICE (Interactive Connectivity Establishment)** protocol, which attempts connections in this order:

1. **Host candidate**: Direct connection (same network)
2. **Server reflexive candidate (STUN)**: Direct connection through NAT
3. **Relay candidate (TURN)**: Relayed connection through TURN server ✅ **Fallback when 1 & 2 fail**

TURN acts as a **last-resort fallback** to ensure connectivity when direct connections are blocked.

### Connection Failure Scenarios

| Scenario                                 | STUN Works? | TURN Required?        |
| ---------------------------------------- | ----------- | --------------------- |
| Open Internet / Full Cone NAT            | ✅ Yes      | ❌ No                 |
| Port Restricted / Address Restricted NAT | ✅ Yes      | ❌ No                 |
| Symmetric NAT                            | ❌ No       | ✅ Yes                |
| Carrier-Grade NAT (CGN)                  | ❌ No       | ✅ Yes                |
| Corporate Firewall (UDP blocked)         | ❌ No       | ✅ Yes (TCP fallback) |

---

## 🛠️ Technical Fundamentals

### ICE, STUN, TURN Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    WebRTC Connection Flow                    │
└─────────────────────────────────────────────────────────────┘

1. STUN (Session Traversal Utilities for NAT)
   ┌──────┐      STUN Request       ┌───────────┐
   │Device│ ───────────────────────>│STUN Server│
   └──────┘ <───────────────────────└───────────┘
              "Your public IP is X"

2. Direct P2P (If STUN succeeds)
   ┌────────┐                     ┌────────┐
   │Device A│ <──────────────────>│Device B│
   └────────┘    Direct Connection└────────┘

3. TURN Relay (If STUN fails - Symmetric NAT/Firewall)
   ┌────────┐     ┌──────────┐     ┌────────┐
   │Device A│<────│TURN Relay│────>│Device B│
   └────────┘     └──────────┘     └────────┘
              All traffic relayed
```

### Ephemeral Credentials Pattern

GADS uses **REST API ephemeral credentials** (draft-uberti-behave-turn-rest) for enhanced security:

**Traditional TURN authentication** (❌ Insecure):

- Static username/password pairs
- Credentials shared across clients
- Difficult to rotate

**Ephemeral credentials** (✅ Secure):

- **Time-limited**: Credentials expire automatically (e.g., 1 hour TTL)
- **Unique per session**: Each WebRTC connection gets fresh credentials
- **Stateless validation**: TURN server validates using HMAC-SHA1 signature
- **No credential storage**: TURN server only stores shared secret

**Credential generation**:

```
Username: <unix_timestamp>:<suffix>
Password: base64(HMAC-SHA1(shared_secret, username))

Example:
Username: 1738000000:gads
Password: rT8K2p3wD9xL5vM1nQ6hE7jF4gA0sC8iO2uY5zK3pB9w=
```

The TURN server validates credentials by:

1. Extracting timestamp from username
2. Checking if timestamp > current_time (not expired)
3. Recomputing HMAC-SHA1(shared_secret, username)
4. Comparing computed signature with provided password

### GADS Architecture Flow

```
┌────────────────────────────────────────────────────────────────┐
│                     GADS TURN Integration                      │
└────────────────────────────────────────────────────────────────┘

  GADS Hub (Admin UI)
       │
       │ 1. Admin configures TURN server + shared secret
       ▼
  /admin/turn-config (API)
       │
       │ 2. Hub stores config + propagates to providers via WebSocket
       ▼
  GADS Provider (Device Hosts)
       │
       │ 3. Provider generates ephemeral credentials
       │    (common/auth/turn_credentials.go)
       ▼
  WebRTC Signaling
       │
       │ 4. Credentials sent to browser/client in ICE config
       ▼
  Browser WebRTC Stack
       │
       │ 5. ICE connection establishment
       │    - Try STUN (direct P2P)
       │    - Fallback to TURN if needed
       ▼
  TURN Server (coturn)
       │
       │ 6. Validates ephemeral credentials
       │    - Checks timestamp expiry
       │    - Verifies HMAC signature
       ▼
  Relay Connection Established ✅
```

---

## ✅ Prerequisites

### Infrastructure Requirements

| Resource      | Minimum | Recommended | Notes                                       |
| ------------- | ------- | ----------- | ------------------------------------------- |
| **CPU**       | 1 core  | 2+ cores    | Relay traffic is CPU-intensive (encryption) |
| **RAM**       | 512 MB  | 2 GB        | ~50 MB per 10 concurrent sessions           |
| **Bandwidth** | 10 Mbps | 100+ Mbps   | **Critical**: All media flows through TURN  |
| **Storage**   | 1 GB    | 5 GB        | For logs and certificates                   |

**Bandwidth calculation example**:

- 1 WebRTC session @ 2 Mbps video = ~2 Mbps relay traffic
- 10 concurrent sessions = ~20 Mbps minimum
- Add 50% overhead for TCP/TLS encapsulation = **30 Mbps required**

### Network Requirements

- **Public IP address**: TURN server must be reachable from the internet
- **Firewall ports** (must be open):

  ```
  TCP/UDP 3478   → TURN server (STUN/TURN)
  TCP/UDP 5349   → TURN server (STUN/TURN over TLS)
  UDP 49152-49652 → Relay ports (recommend 500 ports, not full 16K range)
  ```

- **DNS** (optional but recommended): Domain name for TLS certificates (e.g., `turn.example.com`)

### Software Requirements

- **Docker**: Version 20.10+ ([Install Docker](https://docs.docker.com/engine/install/))
- **Docker Compose**: Version 2.0+ ([Install Compose](https://docs.docker.com/compose/install/))
- **openssl**: For generating shared secrets (pre-installed on most Linux distributions)

**Verification**:

```bash
docker --version          # Should show: Docker version 20.10+
docker-compose --version  # Should show: Docker Compose version 2.0+
openssl version           # Should show: OpenSSL 1.1.1+
```

---

## 🚀 Deployment Step-by-Step

### Step 1: Clone the turn-server Repository

```bash
cd ~/git
git clone https://github.com/YOUR_ORG/turn-server.git  # Replace with actual repo
cd turn-server
```

**Expected directory structure**:

```
turn-server/
├── docker-compose.yaml
├── turnserver.conf
├── turnserver-logs/  (created automatically)
└── README.md
```

---

### Step 2: Generate Shared Secret

**⚠️ CRITICAL**: This secret authenticates all TURN connections. **Never commit it to git**.

```bash
openssl rand -base64 32
```

**Example output**:

```
O5V/O/yvaWZs/UJ5/o5F3+nikg3DjTq2PCeuMRmAjDw=
```

**Save this secret securely**:

- ✅ Use a password manager
- ✅ Store in `.env` file (added to `.gitignore`)
- ✅ Use Docker secrets for production
- ❌ **NEVER** commit to git
- ❌ **NEVER** share in logs/documentation

---

### Step 3: Configure `turnserver.conf`

Edit `turnserver.conf` with your deployment details:

```bash
nano turnserver.conf  # or vim, code, etc.
```

**Required configuration** (replace placeholders):

```ini
# ============================================
# Listening Ports
# ============================================
listening-port=3478          # Standard TURN port
tls-listening-port=5349      # TURN over TLS (optional but recommended)

# ============================================
# IP Configuration
# ============================================
listening-ip=YOUR_PRIVATE_IP       # Server's private IP (e.g., 10.0.0.100)
external-ip=YOUR_PUBLIC_IP         # Server's public IP (e.g., 203.0.113.50)

# ============================================
# Realm and Server Name
# ============================================
realm=turn.example.com      # Domain name or public IP
server-name=turn.example.com

# ============================================
# Authentication (Ephemeral Credentials)
# ============================================
use-auth-secret
static-auth-secret=YOUR_GENERATED_SECRET  # Paste secret from Step 2

# ============================================
# Relay Ports
# ============================================
# Recommendation: Use 500 ports (not full 16K range) for security
min-port=49152
max-port=49652   # 500 ports (49152-49652)

# ============================================
# Performance
# ============================================
fingerprint      # STUN FINGERPRINT (RFC 5389)
no-cli           # Disable CLI (security)

# ============================================
# Logging
# ============================================
log-file=/var/log/turnserver.log
verbose          # Enable detailed logs (disable in production for performance)
log-binding      # Log IP bindings (useful for troubleshooting)
Log-file=stdout  # Also log to Docker stdout

# ============================================
# TLS Configuration (Optional but Recommended)
# ============================================
# Uncomment and configure if using TLS:
# cert=/etc/letsencrypt/live/turn.example.com/fullchain.pem
# pkey=/etc/letsencrypt/live/turn.example.com/privkey.pem

# ============================================
# Peer IP Access Control
# ============================================
# Allow private networks (required for relaying to internal devices)
allowed-peer-ip=10.0.0.0-10.255.255.255        # Class A private
allowed-peer-ip=172.16.0.0-172.31.255.255      # Class B private
allowed-peer-ip=192.168.0.0-192.168.255.255    # Class C private

# Deny special ranges (security)
denied-peer-ip=127.0.0.0-127.255.255.255       # Localhost
denied-peer-ip=0.0.0.0-0.255.255.255           # Current network
denied-peer-ip=169.254.0.0-169.254.255.255     # Link-local
```

**How to find your IPs**:

```bash
# Private IP (listening-ip)
hostname -I | awk '{print $1}'

# Public IP (external-ip)
curl -4 ifconfig.me
```

---

### Step 4: Configure Docker Compose

The `docker-compose.yaml` should look like this:

```yaml
services:
  coturn:
    image: coturn/coturn:latest
    container_name: gads-turn
    network_mode: host # Required for proper IP binding
    restart: unless-stopped
    volumes:
      - ./turnserver.conf:/etc/coturn/turnserver.conf
      - ./turnserver-logs:/var/log:rw
    user: root
    command:
      - "-c"
      - "/etc/coturn/turnserver.conf"
```

**Configuration notes**:

- `network_mode: host`: Required for coturn to bind to multiple ports efficiently
- `restart: unless-stopped`: Auto-restart on failures (except manual stops)
- Volume mounts:
  - `./turnserver.conf` → Container's config
  - `./turnserver-logs` → Persistent logs (survives container restarts)

---

### Step 5: Firewall Configuration

**⚠️ CRITICAL**: Firewall must allow TURN traffic.

#### UFW (Ubuntu/Debian)

```bash
# TURN ports
sudo ufw allow 3478/tcp
sudo ufw allow 3478/udp
sudo ufw allow 5349/tcp
sudo ufw allow 5349/udp

# Relay ports (adjust range if you changed min/max-port)
sudo ufw allow 49152:49652/udp

# Apply changes
sudo ufw reload

# Verify rules
sudo ufw status numbered
```

#### iptables (Generic Linux)

```bash
# TURN ports
sudo iptables -A INPUT -p tcp --dport 3478 -j ACCEPT
sudo iptables -A INPUT -p udp --dport 3478 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 5349 -j ACCEPT
sudo iptables -A INPUT -p udp --dport 5349 -j ACCEPT

# Relay ports
sudo iptables -A INPUT -p udp --dport 49152:49652 -j ACCEPT

# Save rules (Debian/Ubuntu)
sudo iptables-save | sudo tee /etc/iptables/rules.v4

# Save rules (RHEL/CentOS)
sudo service iptables save
```

**Verification**:

```bash
# Check if ports are listening (after deployment)
sudo ss -tulpn | grep -E '3478|5349'
```

**Expected output**:

```
udp   UNCONN  0  0  YOUR_PRIVATE_IP:3478  0.0.0.0:*  users:(("turnserver",pid=1234,fd=5))
tcp   LISTEN  0  128  YOUR_PRIVATE_IP:3478  0.0.0.0:*  users:(("turnserver",pid=1234,fd=6))
```

---

### Step 6: Deploy TURN Server

```bash
cd ~/git/turn-server
docker-compose up -d
```

**Expected output**:

```
[+] Running 1/1
 ✔ Container gads-turn  Started
```

**Verify container is running**:

```bash
docker ps | grep gads-turn
```

**Expected output**:

```
CONTAINER ID   IMAGE                  COMMAND                  STATUS         PORTS     NAMES
abc123def456   coturn/coturn:latest   "/usr/bin/turnserver…"   Up 5 seconds             gads-turn
```

**Check logs**:

```bash
docker logs gads-turn
```

**Expected log entries**:

```
0: : log file opened: /var/log/turnserver.log
0: : Listener address to use: YOUR_PRIVATE_IP:3478
0: : Relay address to use: YOUR_PUBLIC_IP
0: : TURN server ready
```

---

## ⚙️ GADS Hub Configuration

### Step 1: Access Admin UI

1. Open GADS Hub Admin UI: `http://YOUR_HUB_IP:10000/admin`
2. Navigate to **Global Settings**
3. Scroll to **TURN Server Configuration** section

### Step 2: Configure TURN Settings

Fill in the following fields:

| Field                  | Value               | Example                               |
| ---------------------- | ------------------- | ------------------------------------- |
| **Enable TURN Server** | ✅ Checked          | -                                     |
| **TURN Server**        | Hostname or IP      | `turn.example.com` or `203.0.113.50`  |
| **Port**               | TURN listening port | `3478` (default)                      |
| **Shared Secret**      | Secret from Step 2  | `O5V/O/yvaWZs...` (paste full secret) |
| **TTL (seconds)**      | Credential lifetime | `3600` (1 hour recommended)           |

**Security note**: The shared secret is transmitted over HTTPS and stored encrypted in the Hub's database.

### Step 3: Save Configuration

1. Click **Save TURN Config**
2. Wait for confirmation: "TURN configuration saved successfully"
3. Hub automatically propagates config to all connected providers via WebSocket

### API Endpoint (Alternative to UI)

```bash
# Get current TURN config
curl -X GET http://YOUR_HUB_IP:10000/admin/turn-config \
  -H "Authorization: Bearer YOUR_API_TOKEN"

# Set TURN config
curl -X POST http://YOUR_HUB_IP:10000/admin/turn-config \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_TOKEN" \
  -d '{
    "enabled": true,
    "server": "turn.example.com",
    "port": 3478,
    "shared_secret": "YOUR_GENERATED_SECRET",
    "ttl": 3600
  }'
```

### How Configuration Propagates

```
Admin UI → Hub API → Database → WebSocket → Providers → WebRTC Clients
   |          |         |            |           |             |
   Save    Validate   Store      Broadcast   Generate    Use TURN
  Config   Input     Config      to all    Ephemeral   Credentials
                                Providers  Credentials
```

**Propagation timing**:

- **Existing connections**: Use old config (remain active until disconnected)
- **New connections**: Use new config immediately
- **No restart required**: Changes are hot-reloaded

---

## 🧪 Validation and Testing

### Step 1: Container Health Check

```bash
# Check container status
docker ps | grep gads-turn
# Expected: STATUS shows "Up X minutes" (not restarting)

# Check logs for errors
docker logs gads-turn | grep -i error
# Expected: No output (no errors)

# Check resource usage
docker stats gads-turn --no-stream
# Expected: CPU < 10%, MEM < 100 MB (idle)
```

**Status indicators**:

- ✅ **Healthy**: Container up, no errors, low resource usage
- ⚠️ **Warning**: Container restarting, high CPU/memory
- ❌ **Failed**: Container exited, errors in logs

---

### Step 2: Trickle ICE Test (Browser-Based)

**Best tool for quick validation**: [Trickle ICE Test](https://webrtc.github.io/samples/src/content/peerconnection/trickle-ice/)

**Steps**:

1. Open the Trickle ICE test page
2. Remove default servers (click "X" icons)
3. Add your TURN server:

   ```
   TURN or TURNS URI: turn:turn.example.com:3478
   Username: 1738000000:gads  (generate using GADS or manually with HMAC)
   Password: (HMAC-SHA1 signature - see generation below)
   ```

4. Click **Gather candidates**
5. Look for **relay candidates** in results

**Expected output**:

```
✅ relay 203.0.113.50:49152 typ relay raddr 0.0.0.0 rport 0
✅ relay 203.0.113.50:49153 typ relay raddr 0.0.0.0 rport 0
```

**Troubleshooting results**:

- ❌ **No relay candidates**: Firewall blocking, shared secret mismatch, or TURN server down
- ❌ **401 Unauthorized**: Credentials expired or HMAC signature incorrect
- ✅ **Relay candidates present**: TURN server working correctly

---

### Step 3: CLI Test with `turnutils-uclient`

**Install turnutils (on TURN server or testing machine)**:

```bash
# Ubuntu/Debian
sudo apt-get install coturn-utils

# RHEL/CentOS
sudo yum install coturn-utils
```

**Test TURN connectivity**:

```bash
turnutils_uclient \
  -u "1738000000:gads" \
  -w "YOUR_HMAC_PASSWORD" \
  -v \
  turn.example.com
```

**Expected output**:

```
0: IPv4. Connected to TURN server: turn.example.com:3478
0: Allocate request sent
0: Allocate response received: success
0: Refreshing allocation
✅ Total: sent 100 packets, received 100 packets, loss 0%
```

**How to generate test credentials manually** (if needed):

```bash
# 1. Generate username with future timestamp (1 hour from now)
TIMESTAMP=$(($(date +%s) + 3600))
USERNAME="${TIMESTAMP}:gads"

# 2. Generate HMAC-SHA1 password
SECRET="YOUR_GENERATED_SECRET"
PASSWORD=$(echo -n "$USERNAME" | openssl dgst -sha1 -hmac "$SECRET" -binary | base64)

echo "Username: $USERNAME"
echo "Password: $PASSWORD"
```

---

### Step 4: Verify in `chrome://webrtc-internals`

**During an active GADS WebRTC session**:

1. Open Chrome browser
2. Navigate to `chrome://webrtc-internals`
3. Start a WebRTC connection in GADS
4. In webrtc-internals, look for:

```
Stats for connection RTCPeerConnection_1

ICE candidate pairs:
┌─────────────────────────────────────────────────────┐
│ ✅ Type: relay                                      │
│    Local: udp 10.0.0.100:54321                      │
│    Remote: udp 203.0.113.50:49152                   │
│    State: succeeded                                  │
│    Bytes sent: 1,234,567                            │
│    Bytes received: 987,654                          │
└─────────────────────────────────────────────────────┘
```

**Indicators of TURN usage**:

- ✅ Candidate type: **relay** (not "host" or "srflx")
- ✅ Remote address matches TURN server public IP
- ✅ Bytes sent/received increasing (traffic flowing through relay)

---

### Validation Checklist

Use this checklist to confirm proper deployment:

- [ ] Container running without errors (`docker ps`)
- [ ] Ports listening on correct IPs (`ss -tulpn`)
- [ ] Firewall rules allow TURN traffic (`ufw status` or `iptables -L`)
- [ ] Trickle ICE test shows relay candidates
- [ ] `turnutils-uclient` test successful (0% packet loss)
- [ ] GADS Hub shows TURN config saved
- [ ] `chrome://webrtc-internals` shows relay candidates during connection
- [ ] WebRTC connections succeed from restricted networks (test with mobile hotspot/VPN)

---

## 🔧 Troubleshooting

### Symptom: No Relay Candidates in Trickle ICE

**Diagnosis**:

```bash
# 1. Check if TURN server is reachable
nc -zv turn.example.com 3478
# Expected: "succeeded!" or "open"

# 2. Check Docker logs
docker logs gads-turn | tail -50

# 3. Verify firewall allows ports
sudo ufw status | grep -E '3478|5349|49152'
```

**Possible causes**:

- ❌ TURN server unreachable (firewall blocking)
- ❌ Shared secret mismatch between GADS and TURN server
- ❌ Container crashed (check `docker ps` - status should be "Up", not "Restarting")

**Solutions**:

```bash
# Restart container
docker-compose restart

# Check turnserver.conf has correct shared secret
grep static-auth-secret turnserver.conf
# Compare with GADS Hub config

# Verify external IP is correct
curl -4 ifconfig.me
# Compare with external-ip in turnserver.conf
```

---

### Symptom: 401 Unauthorized Errors

**Diagnosis**:

```bash
# Check TURN server logs for auth failures
docker logs gads-turn | grep -i "401\|unauthorized\|auth"
```

**Example log entry**:

```
ERROR: user 1738000000:gads authentication failed
```

**Possible causes**:

- ❌ Credentials expired (timestamp in username is past)
- ❌ Shared secret mismatch
- ❌ HMAC signature incorrectly computed

**Solutions**:

1. **Verify shared secret matches**:

   ```bash
   # TURN server
   grep static-auth-secret ~/git/turn-server/turnserver.conf

   # GADS Hub (check in Admin UI or API)
   curl -X GET http://YOUR_HUB_IP:10000/admin/turn-config
   ```

2. **Regenerate credentials** (GADS does this automatically, but verify TTL):
   - Check GADS Hub TTL setting (default: 3600 seconds = 1 hour)
   - Credentials expire after TTL - this is normal and expected

3. **Test with fresh credentials**:

   ```bash
   # Generate fresh credentials with future timestamp
   TIMESTAMP=$(($(date +%s) + 3600))
   USERNAME="${TIMESTAMP}:gads"
   PASSWORD=$(echo -n "$USERNAME" | openssl dgst -sha1 -hmac "YOUR_SECRET" -binary | base64)

   # Test with turnutils
   turnutils_uclient -u "$USERNAME" -w "$PASSWORD" -v turn.example.com
   ```

---

### Symptom: Connection Established but No Media

**Diagnosis**:

```bash
# Check relay port range is open
sudo ufw status | grep 49152:49652

# Check if relay ports are allocated
docker logs gads-turn | grep "relay"
```

**Possible causes**:

- ❌ Relay ports (49152-49652) blocked by firewall
- ❌ Insufficient bandwidth on TURN server
- ❌ Relay port range too small (increase max-port)

**Solutions**:

```bash
# 1. Open relay port range
sudo ufw allow 49152:49652/udp
sudo ufw reload

# 2. Check bandwidth usage
docker stats gads-turn --no-stream
# If CPU > 80% or MEM > 90%, server is overloaded

# 3. Increase relay port range (if many concurrent users)
# Edit turnserver.conf:
min-port=49152
max-port=50152  # Increased from 49652 (1000 ports total)

# Restart container
docker-compose restart
```

---

### Symptom: High CPU/Memory Usage

**Diagnosis**:

```bash
# Check resource usage
docker stats gads-turn --no-stream

# Check number of active allocations
docker logs gads-turn | grep -c "Allocate request"
```

**Expected resource usage**:

- **Idle**: CPU < 5%, MEM ~50 MB
- **10 concurrent sessions**: CPU 20-40%, MEM 200-500 MB
- **50+ sessions**: CPU 60-80%, MEM 1-2 GB

**Solutions**:

1. **Scale vertically**: Upgrade to larger instance (more CPU/RAM)
2. **Scale horizontally**: Deploy multiple TURN servers (use DNS round-robin)
3. **Optimize config**:
   ```ini
   # Add to turnserver.conf
   max-bps=1000000        # Limit per-session bandwidth (1 Mbps)
   user-quota=10          # Max 10 sessions per user
   total-quota=100        # Max 100 global sessions
   ```

---

### Symptom: Connection Works Locally but Fails Remotely

**Diagnosis**:

```bash
# Test from external network
curl -4 ifconfig.me  # Get your public IP
# Test from different network (mobile hotspot, VPN, etc.)
```

**Possible causes**:

- ❌ `external-ip` in `turnserver.conf` is incorrect (set to private IP instead of public)
- ❌ NAT/router not forwarding ports correctly
- ❌ Cloud firewall rules (AWS Security Groups, GCP Firewall Rules)

**Solutions**:

1. **Verify external IP**:

   ```bash
   # Check current external IP
   curl -4 ifconfig.me

   # Update turnserver.conf
   external-ip=YOUR_ACTUAL_PUBLIC_IP

   # Restart
   docker-compose restart
   ```

2. **Check cloud firewall** (AWS example):
   ```bash
   # Ensure Security Group allows:
   # - TCP/UDP 3478 from 0.0.0.0/0
   # - TCP/UDP 5349 from 0.0.0.0/0
   # - UDP 49152-49652 from 0.0.0.0/0
   ```

---

## 🔒 Security Best Practices

### 1. Shared Secret Rotation

**Recommendation**: Rotate shared secret **quarterly** (every 3 months).

**Rotation process** (zero-downtime):

1. Generate new secret:

   ```bash
   NEW_SECRET=$(openssl rand -base64 32)
   echo "New secret: $NEW_SECRET"
   ```

2. Update TURN server config (keep old secret temporarily):

   ```ini
   # turnserver.conf
   static-auth-secret=OLD_SECRET,NEW_SECRET  # Comma-separated for dual-secret period
   ```

3. Restart TURN server:

   ```bash
   docker-compose restart
   ```

4. Update GADS Hub with new secret (Admin UI or API)

5. Wait for TTL period (e.g., 1 hour) - old credentials expire

6. Remove old secret from TURN config:

   ```ini
   static-auth-secret=NEW_SECRET  # Remove OLD_SECRET
   ```

7. Restart again:
   ```bash
   docker-compose restart
   ```

---

### 2. TLS Setup with Let's Encrypt

**Why TLS?**

- Encrypts TURN traffic (prevents eavesdropping)
- Required for some corporate networks (TCP 3478 blocked, but TCP 443 allowed)
- Recommended for production deployments

**Prerequisites**:

- Domain name pointing to TURN server (e.g., `turn.example.com`)
- Port 80/443 accessible (for Let's Encrypt validation)

**Setup steps**:

```bash
# 1. Install certbot
sudo apt-get install certbot

# 2. Obtain certificate (replace with your domain)
sudo certbot certonly --standalone -d turn.example.com

# 3. Certificates saved to:
# /etc/letsencrypt/live/turn.example.com/fullchain.pem
# /etc/letsencrypt/live/turn.example.com/privkey.pem

# 4. Update turnserver.conf
cert=/etc/letsencrypt/live/turn.example.com/fullchain.pem
pkey=/etc/letsencrypt/live/turn.example.com/privkey.pem

# 5. Update docker-compose.yaml (mount certificates)
volumes:
  - ./turnserver.conf:/etc/coturn/turnserver.conf
  - ./turnserver-logs:/var/log:rw
  - /etc/letsencrypt:/etc/letsencrypt:ro  # Add this line

# 6. Restart
docker-compose restart
```

**Auto-renewal**:

```bash
# Let's Encrypt certs expire after 90 days
# Set up auto-renewal cron job
sudo crontab -e

# Add this line (runs daily at 2am)
0 2 * * * certbot renew --quiet --post-hook "docker-compose -f /path/to/turn-server/docker-compose.yaml restart"
```

**Test TLS**:

```bash
openssl s_client -connect turn.example.com:5349
# Expected: Certificate details displayed, no errors
```

---

### 3. IP Whitelisting

**Use case**: Restrict TURN usage to known GADS networks (prevent public abuse).

**Configuration** (turnserver.conf):

```ini
# Allow only specific source IPs (GADS Hub/Provider networks)
allowed-peer-ip=203.0.113.0-203.0.113.255    # Your GADS network range
allowed-peer-ip=198.51.100.50                # Specific GADS Hub IP

# Deny all other IPs
denied-peer-ip=0.0.0.0-255.255.255.255       # Block everything else
```

**⚠️ Warning**: This restricts which networks can use TURN. Only use if:

- GADS devices are on known static IPs
- You're experiencing abuse/unauthorized usage
- You understand this blocks legitimate users on dynamic IPs

**Alternative** (less restrictive): Use `max-bps` and `user-quota` to rate-limit instead of blocking.

---

### 4. Monitoring and Abuse Detection

**Key metrics to monitor**:

- **Active allocations**: Number of concurrent TURN sessions
- **Bandwidth usage**: Total relay traffic (GB/day)
- **Auth failures**: Rate of 401 errors (indicates brute-force attempts)
- **CPU/Memory**: Resource exhaustion = potential DoS attack

**Simple monitoring with Docker logs**:

```bash
# Count active allocations
docker logs gads-turn | grep -c "allocation created"

# Count auth failures (last 1000 lines)
docker logs gads-turn --tail 1000 | grep -c "authentication failed"

# Watch logs in real-time
docker logs -f gads-turn
```

**Advanced monitoring** (optional - not required for basic deployments):

- **Prometheus + Grafana**: Coturn can export metrics (requires compiling with Prometheus support)
- **Log aggregation**: Ship logs to ELK stack or Splunk
- **Alerting**: Set up alerts for high auth failure rates or bandwidth spikes

**Anti-over-engineering note**: Don't implement complex monitoring unless you have >100 concurrent users or experience abuse. Start with simple Docker logs and scale monitoring as needed.

## Simple Hetzner TURN server setup using coturn

If you do not want to host the TURN server yourself with all the Docker setup and port changes you can use any VPS hosting solution or another cloud provider. I would suggest using Hetzner because of the low prices and the enormous free bandwidth (20TB per month) for each machine.

- Go to [Hetzner](hetzner.com), register and get the smallest Ubuntu server, it has enough hardware for a TURN server configuration
- Connect to the machine using ssh or via the Hetzner UI
- Install `coturn` with `sudo apt update && sudo apt install coturn`
- Edit `/etc/default/coturn` and uncomment the line `TURNSERVER_ENABLED=1` to enable the `coturn` service
- Generate a shared secret with `openssl rand -base64 32`
- Edit `/etc/turnserver.conf` and add configuration similar to this one

```
# Network
listening-port=3478
tls-listening-port=5349
listening-ip=0.0.0.0
external-ip={hetzner-ip-keep-as-is}
min-port=49152
max-port=65535

# Authentication
realm=yourdomain.com
server-name=yourdomain.com
use-auth-secret
static-auth-secret={shared-secret-key from the previous step}

# Security
fingerprint
no-multicast-peers
no-stun-backward-compatibility

# Logging (optional, remove in production)
log-file=/var/log/turnserver.log
verbose
```

- Start and enable the `coturn` service with `sudo systemctl start coturn && sudo systemctl enable coturn`
- Check the service status with `sudo systemctl status coturn` - I did not have to open any firewall ports manually on the Hetzner machine.
- Test the TURN server as explained [here](#step-2-trickle-ice-test-browser-based)

* Follow the hub configuration as explained [here](#️-gads-hub-configuration)

---

## ❓ FAQ

### Q1: Can I use public TURN servers instead of self-hosting?

**A**: Possible but **not recommended** for production:

| Option          | Pros                                      | Cons                                          |
| --------------- | ----------------------------------------- | --------------------------------------------- |
| **Self-hosted** | Full control, no bandwidth limits, secure | Requires maintenance, hosting costs           |
| **Public TURN** | Free, no maintenance                      | Limited bandwidth, unreliable, security risks |

**Security risk**: Public TURN servers can see all relayed traffic. For GADS (device automation), this exposes screen content and user interactions.

**Recommendation**: Self-host for production. Use public TURN only for development/testing.

---

### Q2: How much bandwidth will my TURN server use?

**Calculation**:

```
Bandwidth = (concurrent sessions) × (video bitrate) × 2

Example:
- 10 concurrent sessions
- 2 Mbps video bitrate
- Total: 10 × 2 Mbps × 2 = 40 Mbps

Why ×2? TURN relays traffic bidirectionally (upload + download)
```

**Real-world multiplier**: Add 30-50% overhead for protocol encapsulation, retransmissions, etc.

**Example monthly cost** (AWS EC2 data transfer):

- 40 Mbps continuous = ~13 TB/month
- AWS data transfer: $0.09/GB = ~$1,170/month
- **Recommendation**: Use cloud providers with free egress (Oracle Cloud, Google Cloud Platform free tier)

---

### Q3: What happens if TURN server goes down?

**Answer**: WebRTC connections **will fail** for users behind restrictive NATs (~8-15% of users).

**Mitigation strategies**:

1. **High availability**: Deploy 2+ TURN servers with DNS round-robin
2. **Monitoring**: Set up uptime checks (e.g., UptimeRobot, Pingdom)
3. **Fallback**: Configure multiple TURN servers in GADS (comma-separated):
   ```
   TURN Server: turn1.example.com,turn2.example.com
   ```

**GADS behavior**: If TURN server is unreachable, WebRTC stack tries:

1. Direct P2P (STUN) - works for ~85% of users
2. TURN fallback - fails if server is down
3. Connection fails with "ICE connection failed" error

---

### Q4: Can I run TURN on the same server as GADS Hub?

**A**: Yes, but **not recommended** for production:

**Reasons to avoid**:

- **Resource contention**: TURN relay is CPU/bandwidth intensive
- **Security**: TURN server exposes ports to internet (increases attack surface on Hub)
- **Scaling**: TURN bandwidth needs may exceed Hub's network capacity

**Acceptable for**:

- Development/testing environments
- Small deployments (<10 concurrent users)
- Environments where TURN is rarely used (open networks)

**If you must co-locate**:

```bash
# Limit TURN resource usage
# Add to turnserver.conf:
max-bps=500000      # 500 Kbps per session
total-quota=20      # Max 20 concurrent sessions
```

---

### Q5: Why does GADS use ephemeral credentials instead of static passwords?

**A**: Ephemeral credentials provide **superior security**:

| Static Passwords               | Ephemeral Credentials           |
| ------------------------------ | ------------------------------- |
| ❌ Shared across all users     | ✅ Unique per session           |
| ❌ Never expire                | ✅ Auto-expire after TTL        |
| ❌ Difficult to rotate         | ✅ Automatic rotation           |
| ❌ If leaked, permanent access | ✅ If leaked, expires in 1 hour |
| ❌ Credential storage required | ✅ Stateless validation         |

**TURN REST API specification**: [draft-uberti-behave-turn-rest](https://datatracker.ietf.org/doc/html/draft-uberti-behave-turn-rest-00)

---

### Q6: How do I scale TURN for high user counts?

**Scaling strategies**:

1. **Vertical scaling** (single server):
   - Upgrade to larger instance (more CPU/RAM/bandwidth)
   - Limit: ~500 concurrent sessions per server

2. **Horizontal scaling** (multiple servers):

   ```
   # Option A: DNS round-robin
   turn.example.com → 203.0.113.50  (Server 1)
                    → 203.0.113.51  (Server 2)
                    → 203.0.113.52  (Server 3)

   # Option B: Multiple TURN URIs in GADS config
   TURN Server: turn1.example.com,turn2.example.com,turn3.example.com
   ```

3. **Geographic distribution**:
   - Deploy TURN servers in multiple regions (US, EU, Asia)
   - Use GeoDNS to route users to nearest server
   - Reduces latency (critical for real-time media)

**Cost-effective scaling**:

- Start with 1 server (sufficient for most deployments)
- Monitor bandwidth/CPU usage
- Scale horizontally only when approaching limits (>80% utilization)
