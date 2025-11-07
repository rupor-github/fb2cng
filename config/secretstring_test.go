package config

import (
	"encoding/json"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestSecretString_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   SecretString
		want    string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			want:    "null",
			wantErr: false,
		},
		{
			name:    "non-empty string",
			input:   "my-secret-password",
			want:    `"` + SecretStringValue + `"`,
			wantErr: false,
		},
		{
			name:    "short string",
			input:   "x",
			want:    `"` + SecretStringValue + `"`,
			wantErr: false,
		},
		{
			name:    "long string",
			input:   "this-is-a-very-long-secret-password-that-should-still-be-hidden",
			want:    `"` + SecretStringValue + `"`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.want == "" {
				if got != nil {
					t.Errorf("MarshalJSON() = %v, want nil", got)
				}
			} else {
				if string(got) != tt.want {
					t.Errorf("MarshalJSON() = %s, want %s", got, tt.want)
				}
			}
		})
	}
}

func TestSecretString_MarshalYAML(t *testing.T) {
	tests := []struct {
		name    string
		input   SecretString
		want    any
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			want:    nil,
			wantErr: false,
		},
		{
			name:    "non-empty string",
			input:   "my-secret-api-key",
			want:    SecretStringValue,
			wantErr: false,
		},
		{
			name:    "short string",
			input:   "a",
			want:    SecretStringValue,
			wantErr: false,
		},
		{
			name:    "long string",
			input:   "super-secret-token-12345678901234567890",
			want:    SecretStringValue,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.MarshalYAML()
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("MarshalYAML() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSecretString_JSON_Integration(t *testing.T) {
	type TestStruct struct {
		Username string       `json:"username"`
		Password SecretString `json:"password"`
		APIKey   SecretString `json:"api_key"`
	}

	tests := []struct {
		name     string
		input    TestStruct
		wantJSON string
	}{
		{
			name: "all secrets set",
			input: TestStruct{
				Username: "john",
				Password: "my-password",
				APIKey:   "api-key-123",
			},
			wantJSON: `{"username":"john","password":"\u003csecret\u003e","api_key":"\u003csecret\u003e"}`,
		},
		{
			name: "empty secrets",
			input: TestStruct{
				Username: "jane",
				Password: "",
				APIKey:   "",
			},
			wantJSON: `{"username":"jane","password":null,"api_key":null}`,
		},
		{
			name: "mixed secrets",
			input: TestStruct{
				Username: "bob",
				Password: "secret123",
				APIKey:   "",
			},
			wantJSON: `{"username":"bob","password":"\u003csecret\u003e","api_key":null}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			if string(got) != tt.wantJSON {
				t.Errorf("json.Marshal() = %s, want %s", got, tt.wantJSON)
			}

			// Verify actual secret is not in output
			if len(tt.input.Password) > 0 && string(got) != tt.wantJSON {
				if containsSubstring(string(got), string(tt.input.Password)) {
					t.Error("Marshaled JSON contains actual password")
				}
			}

			if len(tt.input.APIKey) > 0 && string(got) != tt.wantJSON {
				if containsSubstring(string(got), string(tt.input.APIKey)) {
					t.Error("Marshaled JSON contains actual API key")
				}
			}
		})
	}
}

func TestSecretString_YAML_Integration(t *testing.T) {
	type TestStruct struct {
		Username string       `yaml:"username"`
		Password SecretString `yaml:"password"`
		Token    SecretString `yaml:"token"`
	}

	tests := []struct {
		name     string
		input    TestStruct
		wantYAML string
	}{
		{
			name: "all secrets set",
			input: TestStruct{
				Username: "alice",
				Password: "pass123",
				Token:    "token456",
			},
			wantYAML: "username: alice\npassword: <secret>\ntoken: <secret>\n",
		},
		{
			name: "empty secrets",
			input: TestStruct{
				Username: "bob",
				Password: "",
				Token:    "",
			},
			wantYAML: "username: bob\npassword: null\ntoken: null\n",
		},
		{
			name: "one secret set",
			input: TestStruct{
				Username: "charlie",
				Password: "secret-pwd",
				Token:    "",
			},
			wantYAML: "username: charlie\npassword: <secret>\ntoken: null\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := yaml.Marshal(tt.input)
			if err != nil {
				t.Fatalf("yaml.Marshal() error = %v", err)
			}

			if string(got) != tt.wantYAML {
				t.Errorf("yaml.Marshal() = %s, want %s", got, tt.wantYAML)
			}

			// Verify actual secret is not in output
			if len(tt.input.Password) > 0 {
				if containsSubstring(string(got), string(tt.input.Password)) {
					t.Error("Marshaled YAML contains actual password")
				}
			}

			if len(tt.input.Token) > 0 {
				if containsSubstring(string(got), string(tt.input.Token)) {
					t.Error("Marshaled YAML contains actual token")
				}
			}
		})
	}
}

func TestSecretStringValue_Constant(t *testing.T) {
	if SecretStringValue != "<secret>" {
		t.Errorf("SecretStringValue = %s, want <secret>", SecretStringValue)
	}
}

func TestSecretString_NoLeakage(t *testing.T) {
	secret := SecretString("super-secret-password-12345")

	// Test JSON
	jsonBytes, err := json.Marshal(secret)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	jsonStr := string(jsonBytes)
	if containsSubstring(jsonStr, "super-secret") {
		t.Error("Secret leaked in JSON marshaling")
	}
	if containsSubstring(jsonStr, "password-12345") {
		t.Error("Secret leaked in JSON marshaling")
	}

	// Test YAML
	yamlBytes, err := yaml.Marshal(secret)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	yamlStr := string(yamlBytes)
	if containsSubstring(yamlStr, "super-secret") {
		t.Error("Secret leaked in YAML marshaling")
	}
	if containsSubstring(yamlStr, "password-12345") {
		t.Error("Secret leaked in YAML marshaling")
	}
}

func TestSecretString_EmptyString(t *testing.T) {
	var empty SecretString

	// Test JSON with zero value
	jsonBytes, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(jsonBytes) != "null" {
		t.Errorf("Expected 'null' for empty SecretString, got %v", string(jsonBytes))
	}

	// Test YAML with zero value
	yamlInterface, err := empty.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}
	if yamlInterface != nil {
		t.Errorf("Expected nil for empty SecretString, got %v", yamlInterface)
	}
}

func TestSecretString_TypeConversion(t *testing.T) {
	// Test that we can convert to/from string
	original := "my-secret"
	secret := SecretString(original)

	// When used as string, it should be the original value
	asString := string(secret)
	if asString != original {
		t.Errorf("string(secret) = %s, want %s", asString, original)
	}

	// But when marshaled, it should be hidden
	jsonBytes, _ := json.Marshal(secret)
	if containsSubstring(string(jsonBytes), original) {
		t.Error("Secret visible in JSON output")
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()
}
