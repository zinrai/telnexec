# telnexec Design

## Overview

`telnexec` is a command-line tool that executes commands on devices accessible
via Telnet.

The tool launches the system `telnet` client and controls the interactive
session using an expect-style interaction model.

`telnexec` does not implement the Telnet protocol.

## Execution Model

`telnexec` performs the following sequence:

1. start `telnet`
2. wait for a login prompt
3. send the username
4. wait for a password prompt
5. send the login password
6. wait for the device prompt
7. execute commands sequentially
8. exit the session

If a password prompt appears during command execution (for example when
entering `enable` mode), the tool sends the enable password if available.

## Output Streams

`telnexec` uses separate output streams.

| Stream | Content |
|------|------|
| stdout | command output from the remote device |
| stderr | execution logs |

This allows command output to be captured independently from execution logs.

Example:

```bash
telnexec ... > result.txt
```

## Credentials

Credentials are provided through environment variables.

```
TELNET_PASS
TELNET_ENABLE
```

| Variable      | Description               |
| ------------- | ------------------------- |
| TELNET_PASS   | login password            |
| TELNET_ENABLE | enable/configure password |

`TELNET_PASS` must be set.

`TELNET_ENABLE` is optional. If it is not set and a password prompt appears
during command execution, an empty response is sent.

## Prompt Detection

`telnexec` detects prompts using regular expressions.

The following prompts are recognized:

* login prompt
* password prompt
* device prompt

Default patterns are provided but can be overridden using CLI options.

Prompt formats differ between devices, therefore prompt detection relies on
regular expression matching.

## Host Argument

The host parameter is passed directly to the `telnet` command.

`telnexec` does not validate or transform this value.

## Limitations

Because the tool automates an interactive Telnet session:

* prompt detection relies on pattern matching
* device-specific prompt formats may require custom patterns
* command output may include sensitive information

## Non-Goals

`telnexec` does not attempt to:

* implement the Telnet protocol
* replace the system `telnet` client
* provide device-specific automation
* automatically disable device paging
* provide a full network automation framework
