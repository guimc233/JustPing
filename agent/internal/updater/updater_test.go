package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
		isNewer  bool
	}{
		{"v1.0.0", "1.0.0", 0, false},
		{"v1.0.1", "1.0.0", 1, true},
		{"1.0.0", "1.0.1", -1, false},
		{"v2.0.0", "v1.9.9", 1, true},
		{"1.10.0", "1.9.0", 1, true},
		{"1.0.0-rc1", "1.0.0", -1, false},
		{"1.0.0", "1.0.0-rc1", 1, true},
		{"v1.0.1", "unknown", 1, true},
		{"v1.0.1", "dev", 1, true},
	}

	for _, tc := range tests {
		cmp := CompareVersions(tc.v1, tc.v2)
		if tc.v2 != "unknown" && tc.v2 != "dev" {
			if cmp != tc.expected {
				t.Errorf("CompareVersions(%s, %s) = %d, expected %d", tc.v1, tc.v2, cmp, tc.expected)
			}
		}
		newer := IsNewer(tc.v1, tc.v2)
		if newer != tc.isNewer {
			t.Errorf("IsNewer(%s, %s) = %v, expected %v", tc.v1, tc.v2, newer, tc.isNewer)
		}
	}
}

func TestBinaryName(t *testing.T) {
	bin, err := GetBinaryName()
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		if err != nil {
			t.Fatalf("GetBinaryName() error: %v", err)
		}
		if bin == "" {
			t.Fatalf("GetBinaryName() returned empty string")
		}
	}

	// Test cross platform resolutions
	amd64Bin, err := getBinaryNameFor("linux", "amd64")
	if err != nil || amd64Bin != "justping-agent-linux-amd64" {
		t.Errorf("unexpected linux amd64 bin: %s, %v", amd64Bin, err)
	}

	arm64Bin, err := getBinaryNameFor("linux", "arm64")
	if err != nil || arm64Bin != "justping-agent-linux-arm64" {
		t.Errorf("unexpected linux arm64 bin: %s, %v", arm64Bin, err)
	}

	winBin, err := getBinaryNameFor("windows", "amd64")
	if err != nil || winBin != "justping-agent-windows-amd64.exe" {
		t.Errorf("unexpected windows amd64 bin: %s, %v", winBin, err)
	}
}

func TestChecksumParsing(t *testing.T) {
	sampleSums := `
# Checksums for JustPing
b8bf4ccd758cf47bc47631fc66e38e77d13925ccc1974fbc0b27cc62f8c7c3f1  justping-agent-linux-amd64
f6a2a6fb03b90172770157428314b672429371f71448416f78252eba368d0b14 *justping-agent-linux-arm64
`
	h1 := parseChecksum(sampleSums, "justping-agent-linux-amd64")
	if h1 != "b8bf4ccd758cf47bc47631fc66e38e77d13925ccc1974fbc0b27cc62f8c7c3f1" {
		t.Errorf("unexpected parsed checksum: %s", h1)
	}

	h2 := parseChecksum(sampleSums, "justping-agent-linux-arm64")
	if h2 != "f6a2a6fb03b90172770157428314b672429371f71448416f78252eba368d0b14" {
		t.Errorf("unexpected parsed checksum with asterisk: %s", h2)
	}

	h3 := parseChecksum(sampleSums, "nonexistent")
	if h3 != "" {
		t.Errorf("expected empty for missing binary, got %s", h3)
	}
}

func TestCandidateGeneration(t *testing.T) {
	direct := GetDownloadCandidates("guimc233/JustPing", "v1.0.1", false)
	if len(direct) != 1 || direct[0] != "https://github.com/guimc233/JustPing/releases/download/v1.0.1" {
		t.Errorf("unexpected direct candidates: %v", direct)
	}

	mirrors := GetDownloadCandidates("guimc233/JustPing", "1.0.1", true)
	if len(mirrors) <= 1 {
		t.Errorf("expected multiple mirror candidates, got %d", len(mirrors))
	}
	// Last candidate should be direct github
	if mirrors[len(mirrors)-1] != "https://github.com/guimc233/JustPing/releases/download/v1.0.1" {
		t.Errorf("last candidate should be direct github, got %s", mirrors[len(mirrors)-1])
	}
}

func TestBinaryHeaderValidation(t *testing.T) {
	if runtime.GOOS == "linux" {
		elfHeader := []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01}
		if err := validateBinaryHeader(elfHeader); err != nil {
			t.Errorf("valid ELF rejected: %v", err)
		}
		badHeader := []byte{0x00, 'E', 'L', 'F'}
		if err := validateBinaryHeader(badHeader); err == nil {
			t.Errorf("invalid header accepted")
		}
	}
}

func TestMockDownloadAndVerify(t *testing.T) {
	binName, err := GetBinaryName()
	if err != nil {
		t.Skip("skipping on unsupported OS")
	}

	var dummyBinary []byte
	if runtime.GOOS == "linux" {
		dummyBinary = []byte{0x7f, 'E', 'L', 'F', 0x01, 0x02, 0x03, 0x04}
	} else if runtime.GOOS == "windows" {
		dummyBinary = []byte{'M', 'Z', 0x01, 0x02, 0x03, 0x04}
	} else {
		dummyBinary = []byte{0x01, 0x02, 0x03, 0x04}
	}

	sum := sha256.Sum256(dummyBinary)
	expectedHex := hex.EncodeToString(sum[:])
	sumsContent := fmt.Sprintf("%s  %s\n", expectedHex, binName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(sumsContent))
		case "/" + binName:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(dummyBinary)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Override candidate for test
	client := server.Client()
	checksumURL := server.URL + "/SHA256SUMS.txt"
	binaryURL := server.URL + "/" + binName

	sumsData, err := fetchURL(context.Background(), client, checksumURL)
	if err != nil {
		t.Fatalf("fetchURL failed: %v", err)
	}
	hash := parseChecksum(string(sumsData), binName)
	if hash != expectedHex {
		t.Fatalf("hash mismatch: got %s, want %s", hash, expectedHex)
	}

	binData, err := fetchURL(context.Background(), client, binaryURL)
	if err != nil {
		t.Fatalf("fetch binary failed: %v", err)
	}
	if err := validateBinaryHeader(binData); err != nil {
		t.Fatalf("validateBinaryHeader failed: %v", err)
	}
	if sha256Hex(binData) != expectedHex {
		t.Fatalf("sha256Hex mismatch")
	}
}
