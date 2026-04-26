package asset

import "testing"

func TestClassifyTerminalCommand(t *testing.T) {
	testCases := []struct {
		name      string
		command   string
		wantLevel string
		wantCode  string
	}{
		{name: "rm rf", command: "rm -rf /tmp/test", wantLevel: "high", wantCode: "rm_recursive_force"},
		{name: "service stop", command: "systemctl stop nginx", wantLevel: "medium", wantCode: "service_interrupt"},
		{name: "shell wrapper", command: "sudo bash -c 'kubectl delete pod test'", wantLevel: "medium", wantCode: "container_resource_delete"},
		{name: "safe command", command: "ls -lah", wantLevel: "", wantCode: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			match := classifyTerminalCommand(tc.command)
			if tc.wantLevel == "" {
				if match != nil {
					t.Fatalf("expected no match, got %#v", match)
				}
				return
			}
			if match == nil {
				t.Fatalf("expected match for %q", tc.command)
			}
			if match.Rule.Level != tc.wantLevel {
				t.Fatalf("unexpected risk level: got %s want %s", match.Rule.Level, tc.wantLevel)
			}
			if match.Rule.Code != tc.wantCode {
				t.Fatalf("unexpected rule code: got %s want %s", match.Rule.Code, tc.wantCode)
			}
		})
	}
}

func TestTerminalCommandTrackerFeed(t *testing.T) {
	tracker := newTerminalCommandTracker()

	commands := tracker.Feed([]byte("rm -r"))
	if len(commands) != 0 {
		t.Fatalf("expected no completed command, got %v", commands)
	}

	commands = tracker.Feed([]byte("f /tmp/test\r\n"))
	if len(commands) != 1 || commands[0] != "rm -rf /tmp/test" {
		t.Fatalf("unexpected commands after CRLF: %v", commands)
	}

	commands = tracker.Feed([]byte("cat /etc/hostz\bs\r"))
	if len(commands) != 1 || commands[0] != "cat /etc/hosts" {
		t.Fatalf("unexpected commands after backspace handling: %v", commands)
	}
}
