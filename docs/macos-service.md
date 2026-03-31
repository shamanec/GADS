# macOS Service setup
You can set up GADS provider to run as a `launchd` service on macOS. This ensures the provider starts automatically on boot and restarts if it crashes.

## Prerequisites
- Move the `gads` binary to `/usr/local/bin/gads`
- Ensure the binary is executable: `sudo chmod +x /usr/local/bin/gads`
- Grant **Full Disk Access** to `/usr/local/bin/gads` in **System Settings > Privacy & Security**.
- **Important:** If providing only iOS devices on an Intel Mac, it is recommended to uninstall `adb` (Android Debug Bridge) to prevent potential kernel panics and USB instability.

## Create the service file
Create a new file at `/Library/LaunchDaemons/com.gads.provider.plist`:
`sudo nano /Library/LaunchDaemons/com.gads.provider.plist`

Paste the following configuration, adjusting the values for your environment:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "[http://www.apple.com/DTDs/PropertyList-1.0.dtd](http://www.apple.com/DTDs/PropertyList-1.0.dtd)">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.gads.provider</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/gads</string>
        <string>provider</string>
        <string>--hub</string>
        <string>http://___.___.___.___:PORT</string>
        <string>--mongo-db</string>
        <string>___.___.___.___:27017</string>
        <string>--nickname</string>
        <string>YOUR-PROVIDER-NAME</string>
        <string>--log-level</string>
        <string>info</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/gads-provider.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/gads-provider-err.log</string>
    <key>WorkingDirectory</key>
    <string>/Users/admin/gads</string>
</dict>
</plist>
```

## Load the service
To activate the service, you must set the correct system permissions and then bootstrap it into the system domain.

```bash
# Set the correct ownership (root is required for LaunchDaemons)
sudo chown root:wheel /Library/LaunchDaemons/com.gads.provider.plist

# Load and start the service
sudo launchctl bootstrap system /Library/LaunchDaemons/com.gads.provider.plist
```

## Manage the service
You can monitor and control the provider using standard macOS launchctl commands.

## Check status
To verify the service is running, look for a PID (Process ID) in the first column. A 0 in the second column indicates a clean run, while a non-zero number indicates the last exit code.

```bash
sudo launchctl list | grep gads
```

## View logs
The provider outputs all activity to the logs defined in the .plist. This is the best way to troubleshoot device connection issues.

```bash
# Follow the live log
tail -f /var/log/gads-provider.log

# Check for startup errors
tail -f /var/log/gads-provider-err.log
```

## Stop or Restart the service
To stop the provider from running in the background:
```bash
sudo launchctl bootout system /Library/LaunchDaemons/com.gads.provider.plist
```

To apply changes made to the .plist file, run the bootout command above followed by the bootstrap command from the Load the service section.

## Troubleshooting
Input/Output Error: If bootstrap fails with Error 5, it usually means the service is already loaded or the file has a syntax error. Try a full reboot of the Mac.
Permission Denied: Ensure the binary has Full Disk Access and that you used sudo for all launchctl commands.
Device Offline: If the provider is running but devices are offline, check the logs to ensure the provider can reach the Hub's MongoDB port (27017).
