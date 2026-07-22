package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	expect "github.com/google/goexpect"
	"google.golang.org/grpc/codes"
)

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	ip := flag.String("host", "", "target IP address or hostname")
	username := flag.String("user", "", "login username")
	prompt := flag.String("prompt-device", `[#>$%]\s*$`, "regex matching the device prompt")
	login := flag.String("prompt-login", `(?i)(user(name)?|login)\s*:\s*$`, "regex matching the login prompt")
	pass := flag.String("prompt-password", `(?i)pass(word)?\s*:\s*$`, "regex matching the password prompt")
	sec := flag.Int("timeout", 15, "timeout in seconds")
	var commands stringSlice
	flag.Var(&commands, "command", "command to execute (can be specified multiple times)")
	showVersion := flag.Bool("version", false, "Print version information and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: telnexec -host <ip> -user <username> -command <command> [-command <command> ...]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  TELNET_PASS    login password (required)\n")
		fmt.Fprintf(os.Stderr, "  TELNET_ENABLE  privilege escalation password (optional)\n")
		fmt.Fprintf(os.Stderr, "                 - not set: sends empty line on password prompt during commands\n")
		fmt.Fprintf(os.Stderr, "                 - set (including empty): sends the value on password prompt\n")
	}
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	if *ip == "" || *username == "" || len(commands) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	logger := log.New(os.Stderr, "", log.LstdFlags)

	logger.Printf("host=%s user=%s timeout=%ds", *ip, *username, *sec)
	logger.Printf("prompt-device=%s", *prompt)
	logger.Printf("prompt-login=%s", *login)
	logger.Printf("prompt-password=%s", *pass)
	for i, cmd := range commands {
		logger.Printf("command[%d]=%s", i, cmd)
	}

	promptRE, err := regexp.Compile(*prompt)
	if err != nil {
		logger.Fatalf("invalid regex for -prompt-device: %v", err)
	}
	loginRE, err := regexp.Compile(*login)
	if err != nil {
		logger.Fatalf("invalid regex for -prompt-login: %v", err)
	}
	passRE, err := regexp.Compile(*pass)
	if err != nil {
		logger.Fatalf("invalid regex for -prompt-password: %v", err)
	}

	password := os.Getenv("TELNET_PASS")
	if password == "" {
		logger.Fatalf("environment variable TELNET_PASS is not set")
	}

	enablePassword, enableSet := os.LookupEnv("TELNET_ENABLE")
	if enableSet {
		logger.Printf("TELNET_ENABLE is set")
	} else {
		logger.Printf("TELNET_ENABLE is not set")
	}

	timeout := time.Duration(*sec) * time.Second

	if err := run(logger, *ip, *username, password, enablePassword, enableSet, commands, promptRE, loginRE, passRE, timeout); err != nil {
		logger.Fatalf("[FAILED] %s : %v", *ip, err)
	}
}

func run(logger *log.Logger, ip, username, password, enablePassword string, enableSet bool, commands []string, promptRE, loginRE, passRE *regexp.Regexp, timeout time.Duration) error {
	logger.Printf("spawning: telnet %s", ip)
	e, _, err := expect.Spawn(fmt.Sprintf("telnet %s", ip), -1)
	if err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	defer e.Close()

	logger.Printf("starting login sequence")
	if err := doLogin(logger, e, username, password, promptRE, loginRE, passRE, timeout); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	logger.Printf("login successful")

	for i, cmd := range commands {
		logger.Printf("sending command[%d]: %s", i, cmd)
		output, err := sendCommand(logger, e, cmd, enablePassword, enableSet, promptRE, passRE, timeout)
		if err != nil {
			return fmt.Errorf("command %q: %w", cmd, err)
		}
		logger.Printf("command[%d] completed: %d bytes of output", i, len(output))
		fmt.Print(output)
	}

	logger.Printf("sending exit")
	e.Send("exit\r")
	logger.Printf("session finished")
	return nil
}

func doLogin(logger *log.Logger, e *expect.GExpect, username, password string, promptRE, loginRE, passRE *regexp.Regexp, timeout time.Duration) error {
	// Step 1: wait for login prompt
	logger.Printf("waiting for prompt-login")
	raw, matched, err := e.Expect(loginRE, timeout)
	if err != nil {
		logger.Printf("prompt-login not found: raw=%q err=%v", raw, err)
		return err
	}
	logger.Printf("matched prompt-login: %q", matched[0])
	e.Send(username + "\r")

	// Step 2: wait for password prompt
	logger.Printf("waiting for prompt-password")
	raw, matched, err = e.Expect(passRE, timeout)
	if err != nil {
		logger.Printf("prompt-password not found: raw=%q err=%v", raw, err)
		return err
	}
	logger.Printf("matched prompt-password: %q", matched[0])
	e.Send(password + "\r")

	// Step 3: wait for device prompt
	logger.Printf("waiting for prompt-device")
	raw, matched, err = e.Expect(promptRE, timeout)
	if err != nil {
		logger.Printf("prompt-device not found: raw=%q err=%v", raw, err)
		return err
	}
	logger.Printf("matched prompt-device: %q", matched[0])
	return nil
}

func sendCommand(logger *log.Logger, e *expect.GExpect, cmd, enablePassword string, enableSet bool, promptRE, passRE *regexp.Regexp, timeout time.Duration) (string, error) {
	e.Send(cmd + "\r")

	// Determine what to send when a password prompt appears during command execution.
	// - TELNET_ENABLE is set: send its value (may be empty string)
	// - TELNET_ENABLE is not set: send just a carriage return
	passResponse := "\r"
	if enableSet {
		passResponse = enablePassword + "\r"
	}

	logger.Printf("waiting for device prompt or password prompt")
	caser := []expect.Caser{
		&expect.Case{R: passRE, S: passResponse, T: expect.Continue(expect.NewStatus(codes.OK, "pass")), Rt: 1},
		&expect.Case{R: promptRE, T: expect.OK()},
	}
	caseNames := []string{"prompt-password", "prompt-device"}
	raw, matched, idx, err := e.ExpectSwitchCase(caser, timeout)
	if err != nil {
		logger.Printf("command failed: raw=%q err=%v", raw, err)
		return "", err
	}
	logger.Printf("matched %s: %q", caseNames[idx], matched[0])

	lines := strings.Split(raw, "\n")
	if len(lines) > 2 {
		lines = lines[1 : len(lines)-1]
	}
	return strings.Join(lines, "\n") + "\n", nil
}
