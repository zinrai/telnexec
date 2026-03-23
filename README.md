# telnexec

A CLI tool that logs into a network device via Telnet and executes commands.

## Motivation

This tool was created to collect command output from multiple Telnet-only
devices.

Entering the username and password manually for each device was inconvenient
when accessing dozens of hosts.

## Requirements

`telnet` client installed on the system.

## Usage

```
telnexec -host <ip> -user <username> -command <command> [-command <command> ...]
```

Password is passed via the `TELNET_PASS` environment variable.

```
export TELNET_PASS='secret'
telnexec -host 192.168.2.101 -user admin -command "show version"
```

### Multiple commands

```
telnexec -host 192.168.2.101 -user admin \
  -command "set cli screen-length 0" \
  -command "show chassis hardware"
```

### Privilege escalation (enable)

If a password prompt appears during command execution (e.g. after `enable`), the behavior depends on the `TELNET_ENABLE` environment variable:

```
# Device requires no password after enable (no prompt appears)
# TELNET_ENABLE is not needed
telnexec -host 192.168.2.101 -user admin \
  -command "enable" -command "show running-config"

# Device prompts for password but accepts empty input
# Leave TELNET_ENABLE unset (sends empty line)
telnexec -host 192.168.2.101 -user admin \
  -command "enable" -command "show running-config"

# Device requires an enable password
export TELNET_ENABLE='enablesecret'
telnexec -host 192.168.2.101 -user admin \
  -command "enable" -command "show running-config"
```

### Custom prompt pattern

By default, telnexec detects prompts ending with `#`, `>`, `$`, or `%`. For more reliable detection, specify a regex that matches your device's prompt:

```
telnexec -host 192.168.2.101 -user admin \
  -prompt-device 'admin@.+[#>]\s*$' \
  -command "show version"
```

### Custom login/password prompts

Some devices use non-standard prompts for login or password. Override thm with `-prompt-login` and `-prompt-password`:

```
telnexec -host 192.168.2.101 -user admin \
  -prompt-login 'Account:\s*$' \
  -prompt-password 'Secret:\s*$' \
  -command "show version"
```

## Flags

| Flag | Description |
|---|---|
| `-host` | Target IP address or hostname (required) |
| `-user` | Login username (required) |
| `-command` | Command to execute, can be specified multiple times (required) |
| `-prompt-device` | Regex matching the device prompt |
| `-prompt-login` | Regex matching the login prompt |
| `-prompt-password` | Regex matching the password prompt |
| `-timeout` | Timeout in seconds |

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `TELNET_PASS` | Yes | Login password. Used during the initial Telnet login sequence. |
| `TELNET_ENABLE` | No | Privilege escalation password. When a password prompt appears during command execution (e.g. after `enable`): if set, sends its value; if not set, sends an empty line. |

## How it works

1. Spawns a `telnet` process to connect to the target host.
2. Waits for a login prompt, sends the username.
3. Waits for a password prompt, sends the password from `TELNET_PASS`.
4. Waits for the device prompt.
5. Sends each command in order, waits for the prompt after each one, and prints the output.
6. Sends `exit` to close the session.

## License

This project is licensed under the [MIT License](./LICENSE).
