package test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"connectrpc.com/connect"
	petv1 "github.com/example/pets/gen/go/pet/v1"
	"github.com/example/pets/gen/go/pet/v1/petv1connect"
)

func getFreePort(t *testing.T) int {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to resolve tcp addr: %v", err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatalf("failed to listen on tcp addr: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestFauxRPC_PetService(t *testing.T) {
	// Find available port
	port := getFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", port)
	baseURL := fmt.Sprintf("http://%s", serverAddr)

	// Start FauxRPC server pointing to generated protobuf descriptor image
	cmd := exec.Command("fauxrpc", "run", //nolint:gosec // G204: Subprocess launched in test with dynamic port
		"--schema=../gen/image.binpb",
		fmt.Sprintf("--addr=%s", serverAddr),
	)

	if err := cmd.Start(); err != nil {
		t.Skipf("fauxrpc binary not available or failed to start: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
	}()

	// Wait for server to become responsive
	httpClient := &http.Client{Timeout: 2 * time.Second}
	ready := false
	for range 20 {
		time.Sleep(100 * time.Millisecond)
		resp, err := httpClient.Get(baseURL + "/fauxrpc/docs/")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			ready = true
			break
		}
	}
	if !ready {
		t.Fatalf("fauxrpc server did not become ready in time")
	}

	client := petv1connect.NewPetServiceClient(httpClient, baseURL, connect.WithProtoJSON())
	ctx := context.Background()

	t.Run("ListPets via FauxRPC mock", func(t *testing.T) {
		req := connect.NewRequest(&petv1.ListPetsRequest{
			PageSize: 10,
		})
		resp, err := client.ListPets(ctx, req)
		if err != nil {
			t.Fatalf("failed to call ListPets on FauxRPC: %v", err)
		}

		if resp.Msg == nil {
			t.Fatal("expected non-nil response message")
		}
		t.Logf("FauxRPC generated %d mock pets (total count: %d)", len(resp.Msg.Pets), resp.Msg.TotalCount)
	})
}
