package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveServerAddressRequiresExactlyOneSelector(t *testing.T) {
	if _, err := resolveServerAddress("", "", "", 7777); err == nil {
		t.Fatal("expected missing address error")
	}
	if _, err := resolveServerAddress("203.0.113.10", "example.com", "", 7777); err == nil {
		t.Fatal("expected multiple address selector error")
	}
}

func TestResolveServerAddressIPv4(t *testing.T) {
	addr, err := resolveServerAddress("203.0.113.10", "", "", 7777)
	if err != nil {
		t.Fatal(err)
	}
	if addr.Mode != "ip" || addr.Target != "203.0.113.10:7777" {
		t.Fatalf("unexpected address: %#v", addr)
	}
}

func TestResolveServerAddressRejectsIPv6(t *testing.T) {
	if _, err := resolveServerAddress("2001:db8::1", "", "", 7777); err == nil {
		t.Fatal("expected IPv6 rejection")
	}
}

func TestSetINIValueUpdatesAndAppends(t *testing.T) {
	lines := []string{
		"[SavedServers]",
		"LastConnected=old.example:7777",
		"",
		"[Other]",
		"Value=1",
	}

	lines = setINIValue(lines, "SavedServers", "LastConnected", "203.0.113.10:7777")
	lines = setINIValue(lines, "Settings.ModMismatch", "bAutoRestart", "True")
	text := strings.Join(lines, "\n")

	if !strings.Contains(text, "LastConnected=203.0.113.10:7777") {
		t.Fatalf("LastConnected was not updated:\n%s", text)
	}
	if !strings.Contains(text, "[Settings.ModMismatch]\nbAutoRestart=True") {
		t.Fatalf("Settings.ModMismatch was not appended:\n%s", text)
	}
}

func TestWriteModRestartDataRequiresExistingServerModList(t *testing.T) {
	dir := t.TempDir()
	gameDir := filepath.Join(dir, "Conan Exiles")
	if err := os.MkdirAll(filepath.Join(gameDir, "ConanSandbox", "Saved"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := writeModRestartData(gameDir, "203.0.113.10:7777"); err == nil {
		t.Fatal("expected missing servermodlist.txt error")
	}

	serverModList := filepath.Join(gameDir, "ConanSandbox", "servermodlist.txt")
	if err := os.WriteFile(serverModList, []byte("*Example.pak\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeModRestartData(gameDir, "203.0.113.10:7777"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(gameDir, "ConanSandbox", "Saved", "ModRestartData.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got modRestartData
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ServerAddress != "203.0.113.10:7777" || got.ServerPassword != "" {
		t.Fatalf("unexpected restart data: %#v", got)
	}
	if got.ModList == "" || strings.Contains(got.ModList, "\\") {
		t.Fatalf("ModList should be slash-normalized: %#v", got)
	}
}
