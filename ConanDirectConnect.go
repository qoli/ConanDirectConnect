package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const appID = "440900"

var runLog *os.File

type modRestartData struct {
	ServerAddress  string `json:"ServerAddress"`
	ServerPassword string `json:"ServerPassword"`
	ModList        string `json:"ModList"`
}

type serverAddress struct {
	Mode   string
	Input  string
	Target string
}

func main() {
	closeLog := startRunLog()
	defer closeLog()
	logOnly("argv: %s\n", strings.Join(os.Args, " "))
	if cwd, err := os.Getwd(); err == nil {
		logOnly("cwd: %s\n", cwd)
	}

	flag.Usage = usage

	ipArg := flag.String("ip", "", "server IPv4 address; skips DNS lookup")
	domainArg := flag.String("domain", "", "server DNS name; resolves to IPv4 before launch")
	host := flag.String("host", "", "deprecated: server host name or IPv4; prefer -ip or -domain")
	port := flag.Int("port", 7777, "server game port")
	queryPort := flag.Int("query-port", 27015, "deprecated; retained for old shortcuts")
	gameDirFlag := flag.String("game-dir", "", "Conan Exiles install directory")
	gameExeFlag := flag.String("game-exe", "", "ConanSandbox-Win64-Shipping.exe path")
	mode := flag.String("mode", "steam-run", "launch mode: steam-run, continue-session, or launcher-connect")
	noLaunch := flag.Bool("no-launch", false, "write config only")
	flag.Parse()

	if err := run(*ipArg, *domainArg, *host, *port, *queryPort, *gameDirFlag, *gameExeFlag, *mode, *noLaunch); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		logOnly("ERROR: %v\n", err)
		pauseIfInteractive()
		os.Exit(1)
	}
	logOnly("completed successfully\n")
}

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, "Usage:\n")
	fmt.Fprintf(out, "  %s -ip 203.0.113.10 [options]\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(out, "  %s -domain example.com [options]\n\n", filepath.Base(os.Args[0]))
	flag.PrintDefaults()
}

func startRunLog() func() {
	path := runLogPath()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return func() {}
	}
	runLog = file
	logOnly("=== start ===\n")
	logOnly("log path: %s\n", path)
	return func() {
		logOnly("=== end ===\n")
		_ = file.Close()
		runLog = nil
	}
}

func runLogPath() string {
	exe, err := os.Executable()
	if err == nil && exe != "" {
		return filepath.Join(filepath.Dir(exe), "ConanDirectConnect.log")
	}
	return "ConanDirectConnect.log"
}

func say(format string, args ...any) {
	fmt.Printf(format, args...)
	logOnly(format, args...)
}

func logOnly(format string, args ...any) {
	if runLog == nil {
		return
	}
	_, _ = fmt.Fprintf(runLog, "%s ", time.Now().Format("2006-01-02 15:04:05"))
	_, _ = fmt.Fprintf(runLog, format, args...)
}

func run(ipArg, domainArg, legacyHost string, port, queryPort int, explicitGameDir, explicitGameExe, mode string, noLaunch bool) error {
	_ = queryPort

	server, err := resolveServerAddress(ipArg, domainArg, legacyHost, port)
	if err != nil {
		return err
	}
	gameDir, err := resolveGameDir(explicitGameDir, explicitGameExe)
	if err != nil {
		return err
	}

	say("Conan Exiles: %s\n", gameDir)
	say("Address mode: %s\n", server.Mode)
	say("Server: %s -> %s\n", server.Input, server.Target)
	say("Mode: %s\n", mode)

	gameINI := filepath.Join(gameDir, "ConanSandbox", "Saved", "Config", "Windows", "Game.ini")
	if err := updateGameINI(gameINI, server.Target); err != nil {
		return err
	}
	logOnly("updated Game.ini: %s\n", gameINI)

	switch mode {
	case "steam-run":
		return launchViaSteamRun(gameDir, server.Target, noLaunch)
	case "launcher-connect":
		return launchViaFuncomLauncher(gameDir, server.Target, noLaunch)
	case "continue-session":
		return launchViaContinueSession(gameDir, server.Target, noLaunch)
	default:
		return fmt.Errorf("unknown mode %q; use steam-run, continue-session, or launcher-connect", mode)
	}
}

func launchViaSteamRun(gameDir, target string, noLaunch bool) error {
	if err := writeModRestartData(gameDir, target); err != nil {
		return err
	}

	say("Steam: steam://run/%s\n", appID)
	if noLaunch {
		say("No launch requested.\n")
		pauseIfInteractive()
		return nil
	}

	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", "steam://run/"+appID)
	logOnly("starting command: %s %s\n", cmd.Path, strings.Join(cmd.Args[1:], " "))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch Steam run game: %w", err)
	}
	say("Started Steam run game request.\n")
	return nil
}

func launchViaFuncomLauncher(gameDir, target string, noLaunch bool) error {
	launcherPath := filepath.Join(gameDir, "Launcher", "FuncomLauncher.exe")
	if _, err := os.Stat(launcherPath); err != nil {
		return fmt.Errorf("Funcom launcher not found: %s", launcherPath)
	}

	say("Launcher: %s\n", launcherPath)
	say("Arguments: +connect %s\n", target)

	if noLaunch {
		say("No launch requested.\n")
		pauseIfInteractive()
		return nil
	}

	cmd := exec.Command(launcherPath, "+connect", target)
	cmd.Dir = filepath.Dir(launcherPath)
	logOnly("starting command: %s %s %s\n", cmd.Path, cmd.Args[1], cmd.Args[2])
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch Funcom launcher: %w", err)
	}
	say("Started Funcom launcher process: %d\n", cmd.Process.Pid)
	return nil
}

func launchViaContinueSession(gameDir, target string, noLaunch bool) error {
	exePath := filepath.Join(gameDir, "ConanSandbox", "Binaries", "Win64", "ConanSandbox-Win64-Shipping.exe")
	if _, err := os.Stat(exePath); err != nil {
		return fmt.Errorf("Conan executable not found: %s", exePath)
	}

	if err := writeModRestartData(gameDir, target); err != nil {
		return err
	}

	if noLaunch {
		say("No launch requested.\n")
		pauseIfInteractive()
		return nil
	}

	cmd := exec.Command(exePath, "--continuesession")
	cmd.Dir = gameDir
	logOnly("starting command: %s %s\n", cmd.Path, cmd.Args[1])
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch Conan continue-session: %w", err)
	}
	say("Started Conan process: %d\n", cmd.Process.Pid)
	return nil
}

func resolveServerAddress(ipArg, domainArg, legacyHost string, port int) (serverAddress, error) {
	ipArg = strings.TrimSpace(ipArg)
	domainArg = strings.TrimSpace(domainArg)
	legacyHost = strings.TrimSpace(legacyHost)

	provided := 0
	for _, value := range []string{ipArg, domainArg, legacyHost} {
		if value != "" {
			provided++
		}
	}
	if provided == 0 {
		return serverAddress{}, fmt.Errorf("server address is required; pass -ip <IPv4> or -domain <name>")
	}
	if provided > 1 {
		return serverAddress{}, fmt.Errorf("use only one server address option: -ip, -domain, or deprecated -host")
	}

	if ipArg != "" {
		ip, err := parseIPv4Literal(ipArg)
		if err != nil {
			return serverAddress{}, err
		}
		return newServerAddress("ip", ipArg, ip, port), nil
	}

	if domainArg != "" {
		if ip := net.ParseIP(domainArg); ip != nil {
			return serverAddress{}, fmt.Errorf("-domain expects a DNS name; use -ip for numeric addresses")
		}
		ip, err := lookupDomainIPv4(domainArg)
		if err != nil {
			return serverAddress{}, err
		}
		return newServerAddress("domain", domainArg, ip, port), nil
	}

	if ip, err := parseIPv4Literal(legacyHost); err == nil {
		return newServerAddress("host-ip", legacyHost, ip, port), nil
	}
	if ip := net.ParseIP(legacyHost); ip != nil {
		return serverAddress{}, fmt.Errorf("-host must be an IPv4 address or DNS name; IPv6 is not supported: %s", legacyHost)
	}
	ip, err := lookupDomainIPv4(legacyHost)
	if err != nil {
		return serverAddress{}, err
	}
	return newServerAddress("host-domain", legacyHost, ip, port), nil
}

func newServerAddress(mode, input, ip string, port int) serverAddress {
	return serverAddress{
		Mode:   mode,
		Input:  input,
		Target: fmt.Sprintf("%s:%d", ip, port),
	}
}

func parseIPv4Literal(raw string) (string, error) {
	ip := net.ParseIP(raw)
	if ip == nil {
		return "", fmt.Errorf("invalid IPv4 address %q", raw)
	}
	v4 := ip.To4()
	if v4 == nil {
		return "", fmt.Errorf("IPv6 is not supported for Conan direct connect: %s", raw)
	}
	return v4.String(), nil
}

func lookupDomainIPv4(domain string) (string, error) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %s: %w", domain, err)
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address found for %s", domain)
}

func resolveGameDir(explicitGameDir, explicitGameExe string) (string, error) {
	exeRel := filepath.Join("ConanSandbox", "Binaries", "Win64", "ConanSandbox-Win64-Shipping.exe")
	if explicitGameExe != "" {
		if _, err := os.Stat(explicitGameExe); err != nil {
			return "", fmt.Errorf("Conan executable not found: %s", explicitGameExe)
		}
		dir := filepath.Dir(explicitGameExe)
		for i := 0; i < 3; i++ {
			dir = filepath.Dir(dir)
		}
		return dir, nil
	}
	if explicitGameDir != "" {
		if _, err := os.Stat(filepath.Join(explicitGameDir, exeRel)); err != nil {
			return "", fmt.Errorf("Conan executable not found under game-dir: %s", filepath.Join(explicitGameDir, exeRel))
		}
		return explicitGameDir, nil
	}
	if envDir := os.Getenv("CONAN_EXILES_DIR"); envDir != "" {
		if _, err := os.Stat(filepath.Join(envDir, exeRel)); err == nil {
			return envDir, nil
		}
	}

	for _, root := range steamLibraries() {
		dir := filepath.Join(root, "steamapps", "common", "Conan Exiles")
		exe := filepath.Join(dir, exeRel)
		manifest := filepath.Join(root, "steamapps", "appmanifest_"+appID+".acf")
		if exists(manifest) && exists(exe) {
			return dir, nil
		}
	}
	for _, root := range steamLibraries() {
		dir := filepath.Join(root, "steamapps", "common", "Conan Exiles")
		if exists(filepath.Join(dir, exeRel)) {
			return dir, nil
		}
	}
	return "", fmt.Errorf("failed to locate Conan Exiles; pass -game-dir or set CONAN_EXILES_DIR")
}

func steamLibraries() []string {
	roots := steamRoots()
	seen := map[string]bool{}
	var libs []string
	add := func(path string) {
		if path == "" || !exists(path) {
			return
		}
		clean := filepath.Clean(path)
		key := strings.ToLower(clean)
		if !seen[key] {
			seen[key] = true
			libs = append(libs, clean)
		}
	}

	for _, root := range roots {
		add(root)
		vdf := filepath.Join(root, "steamapps", "libraryfolders.vdf")
		data, err := os.ReadFile(vdf)
		if err != nil {
			continue
		}
		re := regexp.MustCompile(`"path"\s+"([^"]+)"`)
		for _, match := range re.FindAllSubmatch(data, -1) {
			add(strings.ReplaceAll(string(match[1]), `\\`, `\`))
		}
	}
	sort.Strings(libs)
	return libs
}

func steamRoots() []string {
	seen := map[string]bool{}
	var roots []string
	add := func(path string) {
		if path == "" || !exists(path) {
			return
		}
		clean := filepath.Clean(path)
		key := strings.ToLower(clean)
		if !seen[key] {
			seen[key] = true
			roots = append(roots, clean)
		}
	}

	for _, item := range [][2]string{
		{`HKCU\Software\Valve\Steam`, "SteamPath"},
		{`HKCU\Software\Valve\Steam`, "InstallPath"},
		{`HKLM\Software\WOW6432Node\Valve\Steam`, "InstallPath"},
		{`HKLM\Software\Valve\Steam`, "InstallPath"},
	} {
		if value := queryRegistry(item[0], item[1]); value != "" {
			add(value)
		}
	}

	for _, path := range []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Steam"),
		filepath.Join(os.Getenv("ProgramFiles"), "Steam"),
		`C:\Program Files (x86)\Steam`,
		`C:\Program Files\Steam`,
		`D:\Program Files (x86)\Steam`,
		`D:\SteamLibrary`,
		`E:\SteamLibrary`,
		`F:\SteamLibrary`,
	} {
		add(path)
	}
	return roots
}

func queryRegistry(key, value string) string {
	cmd := exec.Command("reg", "query", key, "/v", value)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && strings.EqualFold(fields[0], value) {
			return strings.Join(fields[2:], " ")
		}
	}
	return ""
}

func updateGameINI(path, target string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read Game.ini: %w", err)
	}
	backup := fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backup, data, 0644); err != nil {
		return fmt.Errorf("failed to write Game.ini backup: %w", err)
	}

	lines := splitLines(string(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})))
	lines = setINIValue(lines, "SavedServers", "LastConnected", target)
	lines = setINIValue(lines, "SavedServers", "LastPassword", "")
	lines = setINIValue(lines, "Settings.ModMismatch", "AutoSubscribe", "True")
	lines = setINIValue(lines, "Settings.ModMismatch", "AutoConnect", "True")
	lines = setINIValue(lines, "Settings.ModMismatch", "bAutoRestart", "True")
	return os.WriteFile(path, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0644)
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func setINIValue(lines []string, section, key, value string) []string {
	header := "[" + section + "]"
	sectionStart := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			sectionStart = i
			break
		}
	}
	if sectionStart < 0 {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		return append(lines, header, key+"="+value)
	}

	sectionEnd := len(lines)
	for i := sectionStart + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			sectionEnd = i
			break
		}
	}

	prefix := strings.ToLower(key) + "="
	for i := sectionStart + 1; i < sectionEnd; i++ {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(lines[i])), prefix) {
			lines[i] = key + "=" + value
			return lines
		}
	}

	out := append([]string{}, lines[:sectionEnd]...)
	out = append(out, key+"="+value)
	out = append(out, lines[sectionEnd:]...)
	return out
}

func writeModRestartData(gameDir, target string) error {
	path := filepath.Join(gameDir, "ConanSandbox", "Saved", "ModRestartData.json")
	serverModListPath := filepath.Join(gameDir, "ConanSandbox", "servermodlist.txt")
	if !exists(serverModListPath) {
		return fmt.Errorf("servermodlist.txt not found: %s", serverModListPath)
	}
	payload := modRestartData{
		ServerAddress:  target,
		ServerPassword: "",
		ModList:        filepath.ToSlash(serverModListPath),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func exists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func pauseIfInteractive() {
	if os.Getenv("CONAN_DIRECT_CONNECT_NO_PAUSE") != "" {
		return
	}
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
		fmt.Println("Press Enter to close...")
		_, _ = fmt.Scanln()
	}
}
